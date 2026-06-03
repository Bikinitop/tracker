# Visitor ID Validation (`_id` / `cid`) — Design

**Date:** 2026-06-03
**Status:** Approved (pending spec review)
**Branch:** `feat/id-validation`

## Goal

Validate the Matomo visitor-ID parameters `_id` and `cid` against the
16-character hexadecimal format Matomo requires, **without rejecting requests**.
Valid IDs are normalized to lowercase; invalid IDs are demoted into the `extra`
passthrough map (and cleared from their typed field) so downstream session
logic doesn't key on garbage — matching Matomo's "ignore an invalid ID, still
track the hit" behavior.

## Background

Matomo's Tracking API specifies `_id` (visitor ID) and `cid` (visitor ID
override) as 16-char hex strings. Matomo treats an invalid value leniently: the
ID is ignored and the visitor is treated as new — the request is **not**
rejected. This service currently copies `_id`/`cid` verbatim with no validation
(compatibility-audit gap). Because hex is case-insensitive, an unnormalized ID
can also produce two visitor keys for the same visitor downstream.

## Non-Goals

- Rejecting requests with `400` on a bad ID (Matomo doesn't; it would drop
  otherwise-valid hits).
- Enforcing `token_auth` for `cid` (Matomo treats `cid` as a privileged
  override). That remains the existing documented gap; here we validate
  **format only**.
- Generating a new ID when one is missing/invalid — that's downstream
  sessionization's job. We only validate/normalize what the client sent.
- `_id`/`cid` length other than 16 or non-hex alphabets.

## Decisions (settled during brainstorming)

| Topic | Decision |
|-------|----------|
| Invalid behavior | Never reject; clear the typed field and stash the raw value in `extra` |
| Valid behavior | Keep, normalized to lowercase |
| Where | At the edge, in `ParseEvent` (validate once; downstream gets clean keys) |
| Scope | `_id` and `cid` only; format `^[0-9a-fA-F]{16}$` |

## Design

### Validation helper (no regexp — hot path)

```go
// normalizeVisitorID returns the lowercased id and ok=true when v is a
// 16-character hexadecimal string; otherwise ok=false. A single pass validates
// and lowercases, avoiding a regexp allocation on the request path.
func normalizeVisitorID(v string) (string, bool) {
	if len(v) != 16 {
		return "", false
	}
	b := []byte(v)
	for i := range b {
		switch c := b[i]; {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
			// already valid lowercase/digit
		case c >= 'A' && c <= 'F':
			b[i] = c + ('a' - 'A') // normalize to lowercase
		default:
			return "", false
		}
	}
	return string(b), true
}
```

### Stash helper (reuses the `extra` mechanism)

```go
// stashExtra adds k=v to m, allocating m if nil, and returns it.
func stashExtra(m map[string]string, k, v string) map[string]string {
	if m == nil {
		m = make(map[string]string)
	}
	m[k] = v
	return m
}
```

### Integration in `ParseEvent`

Run **after** `e.Extra = collectExtra(params)` (so demotion can extend the same
map) and before/around `detectActionType`. For each of `_id`→`VisitorID` and
`cid`→`VisitorUUID`:

```go
	if e.VisitorID != "" {
		if norm, ok := normalizeVisitorID(e.VisitorID); ok {
			e.VisitorID = norm
		} else {
			e.Extra = stashExtra(e.Extra, "_id", e.VisitorID)
			e.VisitorID = ""
		}
	}
	if e.VisitorUUID != "" {
		if norm, ok := normalizeVisitorID(e.VisitorUUID); ok {
			e.VisitorUUID = norm
		} else {
			e.Extra = stashExtra(e.Extra, "cid", e.VisitorUUID)
			e.VisitorUUID = ""
		}
	}
```

Notes:
- Empty `_id`/`cid` are left empty (a missing visitor ID is normal — not an
  error, not demoted).
- A valid ID never appears in `extra` (it stays in its typed field).
- An invalid ID is preserved (in `extra`) — nothing the client sent is dropped,
  consistent with the passthrough philosophy.
- No struct changes; the reflection drift guard is unaffected (`_id`/`cid`
  remain in `handledParams`).

### Action-type note

`detectActionType` does not depend on `_id`/`cid`, so ordering relative to it is
irrelevant; placing the ID step right after `collectExtra` keeps the
`extra`-related logic together.

## Testing (TDD, table-driven, `go test -race`)

In `internal/tracker/event_test.go`:

1. **Valid lowercase kept:** `_id=0123456789abcdef` → `VisitorID` unchanged, not
   in `Extra`.
2. **Valid uppercase normalized:** `_id=0123456789ABCDEF` →
   `VisitorID=0123456789abcdef`, not in `Extra`.
3. **Invalid length demoted:** `_id=abc` → `VisitorID==""`,
   `Extra["_id"]=="abc"`, `ParseEvent` returns no error.
4. **Invalid non-hex demoted:** `_id=zzzzzzzzzzzzzzzz` (16 non-hex) → cleared +
   in `Extra`.
5. **`cid` validated the same way:** valid normalized; invalid → `VisitorUUID==""`,
   `Extra["cid"]=raw`.
6. **Empty `_id` stays empty:** no `_id` param → `VisitorID==""`, no `_id` in
   `Extra`.
7. **Table-driven `normalizeVisitorID`:** lowercase hex, uppercase hex (→lower),
   mixed case, 15/17 chars, non-hex char, empty → expected (value, ok).

## Verification

- `go build ./... && go vet ./... && go test -race ./...` green.
- Existing tests unaffected (any test sending a non-16-hex `_id`, e.g. `"abc123"`,
  will now see it demoted — check `TestParseEvent_ValidParams` and the bulk/
  handler tests for short sentinel IDs and update expectations if needed).

## Risks / Trade-offs

- **Behavior change for existing callers** sending short/non-hex `_id` sentinels
  (common in tests/examples): such IDs now move to `extra` and the typed field
  is cleared. This is the intended Matomo-faithful behavior; existing in-repo
  tests using sentinel IDs must be updated to reflect it (covered in Verification).
- **`cid` without `token_auth`** is still accepted (format-validated only) — the
  auth gap is unchanged and remains documented.
- **Lowercasing** changes the stored value for uppercase IDs; this is desired
  (stable visitor keys) and matches Matomo's case-insensitive treatment.

## Docs

- `docs/API.md`: update the `_id` row (currently "not validated here") and add a
  short note that `_id`/`cid` are validated as 16-hex, normalized to lowercase,
  and invalid values are forwarded under `extra` (the hit is still tracked).
