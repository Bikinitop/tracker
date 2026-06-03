# Passthrough Params (`extra` map) — Design

**Date:** 2026-06-03
**Status:** Approved (pending spec review)
**Branch:** `feat/passthrough-params`

## Goal

Stop silently dropping Matomo tracking parameters the parser doesn't explicitly
recognize. Capture every unrecognized request parameter into an `extra` map on
the `Event` so it is forwarded in the NATS payload, making the tracker
forward-compatible with current and future Matomo params (e.g. the JS tracker's
`_idvc`, `_idts`, `_viewts`, `_ects`, `_ref`; deprecated `gt_ms`; plugin
families `ma_*`, `cra*`).

## Background

`internal/tracker/ParseEvent` builds an `Event` from a fixed whitelist of ~60
known keys (assigned into struct fields), plus `dimension*` prefixed keys
(into `VisitDimensions`/`ActionDimensions`) and a fixed set of plugin-flag keys
(into `Plugins`). The NATS publisher marshals the `Event` struct to JSON. Any
parameter not in that whitelist is never stored on the event and therefore never
published — a real compatibility gap surfaced by the Matomo compatibility audit.

## Non-Goals

- Enforcing `token_auth` for protected override params (separate concern).
- Validating `_id`/`cid` format.
- Serving a `matomo.php` alias.
- Acting on any forwarded param (sessionization, attribution, etc.) — that
  remains the downstream NATS consumers' job. This change only ensures the data
  is *carried through*.

## Decisions (settled during brainstorming)

| Topic | Decision |
|-------|----------|
| Detection | **Approach A** — explicit `handledParams` set + post-pass, guarded by a drift test |
| JSON shape | **Nested** under a distinct `extra` object |
| Size cap | None (YAGNI) — bounded by rate limiting and Go's form/body limits |

## Design

### `Event.Extra`

Add to the `Event` struct (`internal/tracker/event.go`):

```go
// Extra holds request parameters not otherwise recognized by the parser, so
// nothing the client sends is dropped from the published payload. Empty when
// every parameter was recognized.
Extra map[string]string `json:"extra,omitempty"`
```

Resulting payload shape:

```json
{
  "idsite": "1",
  "rec": "1",
  "action_type": "pageview",
  "extra": { "_idvc": "3", "_ref": "https://google.com", "gt_ms": "120" }
}
```

`omitempty` ensures the key is absent when there are no extras (a nil/empty map
marshals away), so existing consumers see no change unless extras are present.

### `handledParams` set

A package-level set of every parameter key that `ParseEvent` maps to a struct
field:

```go
var handledParams = map[string]struct{}{
    "idsite": {}, "rec": {}, "action_name": {}, "url": {}, "_id": {},
    "rand": {}, "apiv": {}, "urlref": {}, "res": {}, "h": {}, "m": {}, "s": {},
    "ua": {}, "uadata": {}, "lang": {}, "uid": {}, "cid": {}, "new_visit": {},
    "_cvar": {}, "cookie": {}, "_rcn": {}, "_rck": {}, "cvar": {}, "link": {},
    "download": {}, "search": {}, "search_cat": {}, "search_count": {},
    "pv_id": {}, "idgoal": {}, "revenue": {}, "cs": {}, "ca": {},
    "pf_net": {}, "pf_srv": {}, "pf_tfr": {}, "pf_dm1": {}, "pf_dm2": {}, "pf_onl": {},
    "e_c": {}, "e_a": {}, "e_n": {}, "e_v": {},
    "c_n": {}, "c_p": {}, "c_t": {}, "c_i": {},
    "ec_id": {}, "ec_items": {}, "ec_st": {}, "ec_tx": {}, "ec_sh": {}, "ec_dt": {},
    "_pkc": {}, "_pkp": {}, "_pks": {}, "_pkn": {},
    "send_image": {}, "ping": {}, "recMode": {}, "bots": {}, "http_status": {},
    "bw_bytes": {}, "source": {}, "token_auth": {}, "cip": {}, "cdt": {},
    "country": {}, "region": {}, "city": {}, "lat": {}, "long": {}, "debug": {},
    // plugin flags (mapped into Plugins)
    "fla": {}, "java": {}, "dir": {}, "qt": {}, "realp": {}, "pdf": {},
    "wma": {}, "gears": {}, "ag": {},
}
```

This set is the single source of truth for "what the parser already consumes."
It deliberately **includes** the plugin-flag keys and **excludes** `dimension*`
keys (handled by prefix).

### Population (post-pass in `ParseEvent`)

After the event is built (and after the existing plugin / dimension loops),
before `detectActionType`:

```go
extra := make(map[string]string)
for key, val := range params {
    if _, known := handledParams[key]; known {
        continue
    }
    if strings.HasPrefix(key, "dimension") {
        continue // already captured in VisitDimensions/ActionDimensions
    }
    extra[key] = val
}
if len(extra) > 0 {
    e.Extra = extra
}
```

Notes:
- Only non-empty `extra` is assigned, so the JSON stays clean.
- Empty-valued unknown params (`?foo=`) are still forwarded (key present, value
  `""`) — matches the parser's existing behavior of copying values verbatim.
- This is the only change to control flow; the struct literal is untouched.

## Testing (TDD, table-driven, `go test -race`)

In `internal/tracker/event_test.go`:

1. **Unknown params captured:** `?idsite=1&rec=1&_idvc=3&_ref=https://x&ma_id=9`
   → `Extra` contains `_idvc`, `_ref`, `ma_id` with correct values.
2. **Known params excluded:** a request exercising `idsite`, `rec`, `e_c`,
   `url`, a plugin flag (`pdf`), and `dimension3` → `Extra` is nil/empty (none of
   those leak into it).
3. **Empty extra omitted from JSON:** marshal an event built from only known
   params → JSON has no `extra` key.
4. **Extra present in JSON:** marshal an event with an unknown param → JSON
   contains `"extra":{...}`.
5. **Empty-valued unknown param forwarded:** `?idsite=1&rec=1&foo=` → `Extra`
   has `foo` = `""`.
6. **Drift guard:** a test that reflects over / enumerates the explicit param
   keys the parser reads and asserts each is in `handledParams` (so a future
   added field can't silently double-publish into `extra`). Concretely: assert
   that for a request setting every known key to a sentinel, `Extra` is empty.

## Verification

- `go test -race ./...` green (existing + new tests).
- `go vet ./...` clean.
- Manual: `go run ./cmd/tracker` with `NATS_URL=disabled` + `debug=1` is
  unaffected (debug echo unchanged); the `extra` map only affects the published
  JSON, verified via unit tests on the marshaled event.

## Risks / Trade-offs

- **Drift:** adding a new mapped struct field without adding its key to
  `handledParams` would cause that param to appear both in its field and in
  `extra`. Mitigated by the drift-guard test (#6) and a code comment tying the
  set to the struct mapping.
- **Payload size:** a client sending many junk params inflates the `extra` map
  and the NATS message. Accepted — bounded by per-IP rate limiting and Go's
  request/form size limits; a cap can be added later if abuse is observed.
- **Reserved keys:** `requests`/`token_auth` in the *bulk envelope* are handled
  by `ParseBulkRequest` before per-request parsing, so they never reach
  `ParseEvent` as stray params; `token_auth` (per-request) is in `handledParams`.
