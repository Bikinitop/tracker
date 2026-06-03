# Passthrough Params (`extra` map) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Forward every unrecognized Matomo tracking parameter through the NATS payload via a new `Event.Extra` map, instead of silently dropping it.

**Architecture:** `ParseEvent` keeps its existing struct-literal mapping. A package-level `handledParams` set lists every key the parser already consumes; a post-pass loop puts any param not in that set (and not `dimension*`-prefixed) into `Event.Extra`, which marshals to a nested `"extra"` JSON object (omitted when empty). A drift-guard test prevents a future mapped field from double-publishing.

**Tech Stack:** Go 1.25, standard library only. Tests are table-driven with `encoding/json` for the marshal assertions; race detector required.

**Design doc:** `docs/superpowers/specs/2026-06-03-passthrough-params-design.md`

**Conventions:** TDD (test first), `go test -race ./...`, tests live beside code (`internal/tracker/event_test.go`). Module path `github.com/bikinitop/tracker`.

---

## File structure

```
internal/tracker/event.go        # MODIFY: add Extra field, handledParams set, post-pass loop
internal/tracker/event_test.go   # MODIFY: add passthrough + drift-guard tests
docs/API.md                      # MODIFY: document the extra passthrough
```

---

## Task 1: `Event.Extra` passthrough map

**Files:**
- Modify: `internal/tracker/event.go`
- Test: `internal/tracker/event_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tracker/event_test.go`:

```go
func TestParseEvent_CapturesUnknownParamsInExtra(t *testing.T) {
	params := map[string]string{
		"idsite": "1", "rec": "1",
		"_idvc": "3", "_ref": "https://google.com", "ma_id": "9",
	}
	e, err := ParseEvent(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"_idvc": "3", "_ref": "https://google.com", "ma_id": "9"}
	for k, v := range want {
		if e.Extra[k] != v {
			t.Errorf("Extra[%q] = %q, want %q", k, e.Extra[k], v)
		}
	}
	if len(e.Extra) != len(want) {
		t.Errorf("Extra has %d entries, want %d: %v", len(e.Extra), len(want), e.Extra)
	}
}

func TestParseEvent_KnownParamsNotInExtra(t *testing.T) {
	params := map[string]string{
		"idsite": "1", "rec": "1", "url": "http://x", "e_c": "cat", "e_a": "act",
		"pdf": "1", "dimension3": "v", "token_auth": "t",
	}
	e, err := ParseEvent(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(e.Extra) != 0 {
		t.Errorf("expected no extras for known params, got %v", e.Extra)
	}
}

func TestParseEvent_EmptyValuedUnknownParamForwarded(t *testing.T) {
	e, err := ParseEvent(map[string]string{"idsite": "1", "rec": "1", "foo": ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v, ok := e.Extra["foo"]
	if !ok || v != "" {
		t.Errorf("expected Extra[foo]=\"\" present, got ok=%v v=%q", ok, v)
	}
}

func TestEvent_ExtraOmittedFromJSONWhenEmpty(t *testing.T) {
	e, _ := ParseEvent(map[string]string{"idsite": "1", "rec": "1"})
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "\"extra\"") {
		t.Errorf("did not expect 'extra' key in JSON: %s", b)
	}
}

func TestEvent_ExtraPresentInJSON(t *testing.T) {
	e, _ := ParseEvent(map[string]string{"idsite": "1", "rec": "1", "_idvc": "3"})
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), "\"extra\"") || !strings.Contains(string(b), "\"_idvc\":\"3\"") {
		t.Errorf("expected extra with _idvc in JSON: %s", b)
	}
}

// Drift guard: every key the parser maps to a field must be in handledParams,
// so a newly added field can never silently double-publish into Extra. Setting
// every handled key to a sentinel must leave Extra empty.
func TestParseEvent_HandledParamsCoverAllMappedKeys(t *testing.T) {
	params := make(map[string]string)
	for k := range handledParams {
		params[k] = "x"
	}
	params["idsite"] = "1"
	params["rec"] = "1"
	e, err := ParseEvent(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(e.Extra) != 0 {
		t.Errorf("handledParams missing keys; these leaked into Extra: %v", e.Extra)
	}
}
```

Ensure `event_test.go` imports `encoding/json` and `strings` (add them to the import block if not already present).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tracker/ -run 'Extra|HandledParams|UnknownParam'`
Expected: FAIL — `e.Extra undefined` and `undefined: handledParams`.

- [ ] **Step 3: Add the `Extra` field to the `Event` struct**

In `internal/tracker/event.go`, inside the `Event` struct, immediately after the
bulk-tracking field block (the `BulkRequests []string` field near the end of the
struct), add:

```go
	// Extra holds request parameters not otherwise recognized by the parser, so
	// nothing the client sends is dropped from the published payload. Empty when
	// every parameter was recognized.
	Extra map[string]string `json:"extra,omitempty"`
```

- [ ] **Step 4: Add the `handledParams` set**

In `internal/tracker/event.go`, add this package-level declaration (e.g. just
above `func ParseEvent`):

```go
// handledParams lists every parameter key ParseEvent maps to a struct field
// (including the plugin-flag keys). Keys NOT in this set, and not prefixed with
// "dimension", are forwarded verbatim in Event.Extra. Keep this in sync with the
// field mapping in ParseEvent — the TestParseEvent_HandledParamsCoverAllMappedKeys
// drift guard enforces it.
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

- [ ] **Step 5: Add the post-pass population loop**

In `internal/tracker/event.go`, in `ParseEvent`, find the custom-dimensions loop
that ends just before `e.ActionType = detectActionType(params)`:

```go
	// Parse custom dimensions
	e.VisitDimensions = make(map[string]string)
	e.ActionDimensions = make(map[string]string)
	for key, val := range params {
		if strings.HasPrefix(key, "dimension") {
			if e.CustomAction == "1" {
				e.ActionDimensions[key] = val
			} else {
				e.VisitDimensions[key] = val
			}
		}
	}

	e.ActionType = detectActionType(params)
```

Insert the passthrough loop between the dimensions loop and the `detectActionType`
line, so that block becomes:

```go
	// Parse custom dimensions
	e.VisitDimensions = make(map[string]string)
	e.ActionDimensions = make(map[string]string)
	for key, val := range params {
		if strings.HasPrefix(key, "dimension") {
			if e.CustomAction == "1" {
				e.ActionDimensions[key] = val
			} else {
				e.VisitDimensions[key] = val
			}
		}
	}

	// Forward any parameter the parser did not otherwise capture, so nothing the
	// client sends is dropped from the published payload.
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

	e.ActionType = detectActionType(params)
```

- [ ] **Step 6: Run the tests to verify they pass (race detector)**

Run: `go test -race ./internal/tracker/`
Expected: PASS (existing tests + the 6 new ones).

- [ ] **Step 7: Run the full suite + vet**

Run: `go build ./... && go vet ./... && go test -race ./...`
Expected: all packages PASS; vet clean. (The publisher already marshals the
`Event` struct, so `extra` flows to NATS with no publisher change.)

- [ ] **Step 8: Commit**

```bash
git add internal/tracker/event.go internal/tracker/event_test.go
git commit -m "feat(tracker): forward unrecognized params via Event.Extra"
```
Append on its own line after a blank line:
Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>

---

## Task 2: Document the passthrough in API.md

**Files:**
- Modify: `docs/API.md`

- [ ] **Step 1: Update the "Common parameters" preamble**

In `docs/API.md`, find this paragraph:

```markdown
The parser recognizes the broad subset of Matomo parameters listed below and
maps them into the published event. Parameters outside this set (and outside the
`dimension{N}` / plugin-flag conventions) are accepted by the request but are
**not** carried into the NATS payload. The most-used recognized parameters:
```

Replace it with:

```markdown
The parser maps the parameters listed below into named fields of the published
event. Any parameter outside this set (and outside the `dimension{N}`
convention) is still forwarded — it is collected verbatim into an `extra` object
on the event, so nothing the client sends is dropped. The most-used named
parameters:
```

- [ ] **Step 2: Update the NATS output section**

In `docs/API.md`, find:

```markdown
NATS wildcard characters (`.`, `*`, `>`) in the site ID / action type are
sanitized so they cannot break subject routing. The payload is the parsed event
(the recognized fields above) serialized as JSON.
```

Replace with:

```markdown
NATS wildcard characters (`.`, `*`, `>`) in the site ID / action type are
sanitized so they cannot break subject routing. The payload is the parsed event
serialized as JSON: recognized parameters appear as named fields, and any other
parameters the client sent appear under an `extra` object, e.g.:

\`\`\`json
{ "idsite": "1", "rec": "1", "action_type": "pageview",
  "extra": { "_idvc": "3", "_ref": "https://google.com" } }
\`\`\`
```

(Note: write the JSON fence with real triple backticks, not the escaped form shown here.)

- [ ] **Step 3: Commit**

```bash
git add docs/API.md
git commit -m "docs: document the extra passthrough object in API.md"
```
Append on its own line after a blank line:
Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>

---

## Final verification

- [ ] **Full suite green with race detector**

Run: `go build ./... && go vet ./... && go test -race ./...`
Expected: all packages PASS, vet clean.

---

## Self-review notes (spec coverage)

- `Event.Extra` field + `omitempty` → Task 1 Step 3.
- `handledParams` set (matches the mapping) → Task 1 Step 4; drift guard → test in Step 1.
- Post-pass excludes handled keys and `dimension*` → Task 1 Step 5.
- Only non-empty `extra` assigned (clean JSON) → Step 5 + omit test.
- Empty-valued unknown param forwarded → test in Step 1.
- Publisher unchanged (marshals struct) → noted in Step 7.
- Docs updated → Task 2.
