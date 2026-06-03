# Visitor ID Validation (`_id` / `cid`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Validate `_id`/`cid` as 16-char hex, normalize valid ones to lowercase, and demote invalid ones into the `extra` passthrough map — never rejecting the request.

**Architecture:** Two small helpers (`normalizeVisitorID` — single-pass hex check + lowercase, no regexp; `stashExtra` — lazy-init map insert) and a short block in `ParseEvent`, run right after `collectExtra`, that normalizes `VisitorID`/`VisitorUUID` or moves the raw value to `Event.Extra`.

**Tech Stack:** Go 1.25, standard library only. Table-driven tests; race detector required.

**Design doc:** `docs/superpowers/specs/2026-06-03-id-validation-design.md`

**Conventions:** TDD, tests in `internal/tracker/event_test.go`, `go test -race ./...`. Module `github.com/bikinitop/tracker`.

**Pre-checked:** The only existing test touching `_id`/`cid` (`event_test.go:92,94,136`) uses `"abc123def4567890"`, which is already valid lowercase 16-hex — it passes through unchanged, so no existing test needs updating.

---

## File structure

```
internal/tracker/event.go        # MODIFY: add normalizeVisitorID + stashExtra helpers; validate _id/cid in ParseEvent
internal/tracker/event_test.go   # MODIFY: add validation tests
docs/API.md                      # MODIFY: document _id/cid validation + normalization
```

---

## Task 1: `_id` / `cid` validation in ParseEvent

**Files:**
- Modify: `internal/tracker/event.go`
- Test: `internal/tracker/event_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/tracker/event_test.go`:

```go
func TestNormalizeVisitorID(t *testing.T) {
	cases := []struct {
		in      string
		wantVal string
		wantOK  bool
	}{
		{"0123456789abcdef", "0123456789abcdef", true},
		{"0123456789ABCDEF", "0123456789abcdef", true}, // uppercase normalized
		{"AbCdEf0123456789", "abcdef0123456789", true}, // mixed case
		{"abc123def4567890", "abc123def4567890", true}, // existing test value
		{"abc", "", false},                             // too short
		{"0123456789abcdef0", "", false},               // 17 chars
		{"zzzzzzzzzzzzzzzz", "", false},                // 16 non-hex
		{"", "", false},                                // empty
	}
	for _, c := range cases {
		gotVal, gotOK := normalizeVisitorID(c.in)
		if gotVal != c.wantVal || gotOK != c.wantOK {
			t.Errorf("normalizeVisitorID(%q) = (%q,%v), want (%q,%v)",
				c.in, gotVal, gotOK, c.wantVal, c.wantOK)
		}
	}
}

func TestParseEvent_ValidVisitorIDNormalized(t *testing.T) {
	e, err := ParseEvent(map[string]string{
		"idsite": "1", "rec": "1",
		"_id": "0123456789ABCDEF", "cid": "ABCDEF0123456789",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.VisitorID != "0123456789abcdef" {
		t.Errorf("VisitorID = %q, want lowercased", e.VisitorID)
	}
	if e.VisitorUUID != "abcdef0123456789" {
		t.Errorf("VisitorUUID = %q, want lowercased", e.VisitorUUID)
	}
	if _, ok := e.Extra["_id"]; ok {
		t.Errorf("valid _id must not be in Extra")
	}
	if _, ok := e.Extra["cid"]; ok {
		t.Errorf("valid cid must not be in Extra")
	}
}

func TestParseEvent_InvalidVisitorIDDemotedToExtra(t *testing.T) {
	e, err := ParseEvent(map[string]string{
		"idsite": "1", "rec": "1",
		"_id": "abc", "cid": "zzzzzzzzzzzzzzzz",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.VisitorID != "" {
		t.Errorf("invalid _id should be cleared, got %q", e.VisitorID)
	}
	if e.VisitorUUID != "" {
		t.Errorf("invalid cid should be cleared, got %q", e.VisitorUUID)
	}
	if e.Extra["_id"] != "abc" {
		t.Errorf("raw invalid _id should be in Extra, got %q", e.Extra["_id"])
	}
	if e.Extra["cid"] != "zzzzzzzzzzzzzzzz" {
		t.Errorf("raw invalid cid should be in Extra, got %q", e.Extra["cid"])
	}
}

func TestParseEvent_EmptyVisitorIDStaysEmpty(t *testing.T) {
	e, err := ParseEvent(map[string]string{"idsite": "1", "rec": "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.VisitorID != "" {
		t.Errorf("expected empty VisitorID")
	}
	if _, ok := e.Extra["_id"]; ok {
		t.Errorf("absent _id must not appear in Extra")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tracker/ -run 'NormalizeVisitorID|VisitorID'`
Expected: FAIL — `undefined: normalizeVisitorID`.

- [ ] **Step 3: Add the helper functions**

In `internal/tracker/event.go`, add these two functions (e.g. just after the
`collectExtra` function):

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
			// valid digit / lowercase hex
		case c >= 'A' && c <= 'F':
			b[i] = c + ('a' - 'A') // normalize to lowercase
		default:
			return "", false
		}
	}
	return string(b), true
}

// stashExtra adds k=v to m, allocating m if nil, and returns it.
func stashExtra(m map[string]string, k, v string) map[string]string {
	if m == nil {
		m = make(map[string]string)
	}
	m[k] = v
	return m
}
```

- [ ] **Step 4: Validate `_id`/`cid` in ParseEvent**

In `internal/tracker/event.go`, find the passthrough line added previously:

```go
	// Forward any parameter the parser did not otherwise capture, so nothing the
	// client sends is dropped from the published payload.
	e.Extra = collectExtra(params)

	e.ActionType = detectActionType(params)
```

Insert the visitor-ID validation between `collectExtra` and `detectActionType`,
so it becomes:

```go
	// Forward any parameter the parser did not otherwise capture, so nothing the
	// client sends is dropped from the published payload.
	e.Extra = collectExtra(params)

	// Validate visitor IDs: keep valid 16-hex values (lowercased) and demote
	// invalid ones into Extra so a bad ID is forwarded, not trusted as a visitor
	// key. Matches Matomo's "ignore invalid ID, still track" behavior.
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

	e.ActionType = detectActionType(params)
```

- [ ] **Step 5: Run the tests to verify they pass (race detector)**

Run: `go test -race ./internal/tracker/`
Expected: PASS (existing tests + the 4 new ones). Note `TestParseEvent_ValidParams`
still passes because its `_id`/`cid` value `"abc123def4567890"` is valid 16-hex.

- [ ] **Step 6: Run the full suite + vet**

Run: `go build ./... && go vet ./... && go test -race ./...`
Expected: all packages PASS, vet clean. (The publisher marshals the Event
struct, so the normalized IDs / demoted extras flow to NATS with no other change.)

- [ ] **Step 7: Commit**

```bash
git add internal/tracker/event.go internal/tracker/event_test.go
git commit -m "feat(tracker): validate and normalize _id/cid, demote invalid to extra"
```
Append on its own line after a blank line:
Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>

---

## Task 2: Document `_id`/`cid` validation in API.md

**Files:**
- Modify: `docs/API.md`

- [ ] **Step 1: Update the `_id` row**

In `docs/API.md`, find:

```markdown
| `_id` | Visitor ID (Matomo uses a 16-char hex string; not validated here) |
```

Replace with:

```markdown
| `_id` | Visitor ID. Validated as a 16-char hex string, normalized to lowercase. An invalid value is moved to `extra` and the visitor is treated as new (the hit is still tracked). |
```

- [ ] **Step 2: Update the `cid` row**

In `docs/API.md`, find:

```markdown
| `cid` | Visitor UUID override |
```

Replace with:

```markdown
| `cid` | Visitor ID override. Validated/normalized like `_id` (format only; `token_auth` is not enforced — see the security note below). |
```

- [ ] **Step 3: Add a note under the "Common parameters" preamble**

In `docs/API.md`, immediately after the "Common parameters" preamble paragraph
(the one ending "The most-used named parameters:"), add a new paragraph:

```markdown
> **Visitor IDs:** `_id` and `cid` must be 16-character hex strings. Valid
> values are normalized to lowercase; an invalid value is forwarded under
> `extra` (e.g. `extra._id`) and the typed field is left empty, so downstream
> session logic doesn't key on a malformed ID. The request is still tracked.
```

- [ ] **Step 4: Commit**

```bash
git add docs/API.md
git commit -m "docs: document _id/cid validation and normalization"
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

- 16-hex validation (`^[0-9a-fA-F]{16}$`) via single-pass helper → Task 1 Step 3.
- Valid → lowercase normalized → `normalizeVisitorID` + Step 4; tested in `TestParseEvent_ValidVisitorIDNormalized`.
- Invalid → cleared + raw in `extra`, no rejection → Step 4; tested in `TestParseEvent_InvalidVisitorIDDemotedToExtra`.
- Empty stays empty → Step 4 guard; tested in `TestParseEvent_EmptyVisitorIDStaysEmpty`.
- Applies to both `_id` and `cid` → both blocks in Step 4.
- Runs after `collectExtra` so demotion extends the same map → Step 4 placement.
- No struct change → drift guard unaffected (not modified).
- Docs → Task 2.
- `cid` token_auth gap unchanged → noted in cid doc row.
