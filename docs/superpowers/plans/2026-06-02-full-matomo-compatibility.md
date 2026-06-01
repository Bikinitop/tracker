# Full Matomo Tracking API Compatibility Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Achieve full compatibility with the Matomo Tracking HTTP API by implementing all missing query parameters, response formats, and tracking modes.

**Architecture:** Extend the `Event` struct to capture all Matomo parameters. Add action type detection (pageview, event, goal, outlink, download, search, ecommerce, content, ping). Support bulk tracking via JSON POST. Add debug mode. Maintain backward compatibility with existing NATS publishing.

**Tech Stack:** Go standard library, `github.com/nats-io/nats.go`

---

## Chunk 1: Enhanced Event Model — Core Parameters

**Goal:** Extend `Event` struct with all Matomo parameters grouped by category. Keep JSON tags matching Matomo query parameter names.

**Files:**
- Modify: `internal/tracker/event.go`
- Modify: `internal/tracker/event_test.go`

### Task 1.1: Expand Event struct with all Matomo parameters

- [ ] **Step 1: Write failing test for new fields**

Create test in `internal/tracker/event_test.go`:

```go
func TestParseEvent_FullMatomoCompatibility(t *testing.T) {
	params := map[string]string{
		"idsite": "1", "rec": "1",
		"action_name": "Home", "url": "https://example.com",
		"_id": "abc123def4567890", "uid": "user1", "cid": "abc123def4567890",
		"res": "1920x1080", "lang": "en-US", "ua": "Mozilla/5.0",
		"urlref": "https://google.com",
		"h": "14", "m": "30", "s": "45",
		"rand": "123456", "apiv": "1",
		"e_c": "Video", "e_a": "Play", "e_n": "Intro", "e_v": "10.5",
		"idgoal": "2", "revenue": "99.99",
		"search": "query", "search_cat": "Products", "search_count": "42",
		"link": "https://outlink.com", "download": "https://example.com/file.pdf",
		"cvar": `{"1":["OS","Mac"]}`, "_cvar": `{"2":["Browser","Chrome"]}`,
		"pf_net": "100", "pf_srv": "200", "pf_tfr": "50",
		"pf_dm1": "300", "pf_dm2": "150", "pf_onl": "400",
		"ec_id": "ORDER-123", "ec_st": "80.00", "ec_tx": "5.00",
		"ec_sh": "10.00", "ec_dt": "5.00",
		"ec_items": `[["SKU1","Item1","Cat1",10.00,2]]`,
		"c_n": "AdBanner", "c_p": "/img/ad.png", "c_t": "https://ad.com", "c_i": "click",
		"dimension1": "value1", "dimension2": "value2",
		"send_image": "0", "ping": "1", "bots": "1", "recMode": "2",
		"cdt": "2024-01-15 10:30:00", "country": "us", "city": "NYC",
		"token_auth": "abc123",
	}

	event, err := ParseEvent(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.SiteID != "1" { t.Errorf("SiteID mismatch") }
	if event.ActionName != "Home" { t.Errorf("ActionName mismatch") }
	if event.EventCategory != "Video" { t.Errorf("EventCategory mismatch") }
	if event.EventAction != "Play" { t.Errorf("EventAction mismatch") }
	if event.GoalID != "2" { t.Errorf("GoalID mismatch") }
	if event.Revenue != "99.99" { t.Errorf("Revenue mismatch") }
	if event.SearchKeyword != "query" { t.Errorf("SearchKeyword mismatch") }
	if event.Ping != "1" { t.Errorf("Ping mismatch") }
	if event.SendImage != "0" { t.Errorf("SendImage mismatch") }
	if event.BulkRequests != nil { t.Errorf("BulkRequests should be nil for single request") }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/tracker/ -v -run TestParseEvent_FullMatomoCompatibility
```
Expected: FAIL — fields like `EventCategory`, `GoalID`, etc. don't exist

- [ ] **Step 3: Expand Event struct with all Matomo fields**

Replace `internal/tracker/event.go`:

```go
package tracker

import (
	"encoding/json"
	"errors"
	"strings"
)

// Event represents a Matomo tracking event to be published to NATS
type Event struct {
	// Required
	SiteID string `json:"idsite"`
	Rec    string `json:"rec"`

	// Recommended
	ActionName string `json:"action_name,omitempty"`
	URL        string `json:"url,omitempty"`
	VisitorID  string `json:"_id,omitempty"`
	Rand       string `json:"rand,omitempty"`
	APIVersion string `json:"apiv,omitempty"`

	// User info
	Referrer    string `json:"urlref,omitempty"`
	Resolution  string `json:"res,omitempty"`
	Hour        string `json:"h,omitempty"`
	Minute      string `json:"m,omitempty"`
	Second      string `json:"s,omitempty"`
	UserAgent   string `json:"ua,omitempty"`
	ClientHints string `json:"uadata,omitempty"`
	Language    string `json:"lang,omitempty"`
	UserID      string `json:"uid,omitempty"`
	VisitorUUID string `json:"cid,omitempty"`
	NewVisit    string `json:"new_visit,omitempty"`
	CustomVars  string `json:"_cvar,omitempty"`
	Cookie      string `json:"cookie,omitempty"`

	// Plugins (flash, java, director, quicktime, realplayer, pdf, wma, gears, silverlight)
	Plugins map[string]string `json:"plugins,omitempty"`

	// Acquisition
	CampaignName   string `json:"_rcn,omitempty"`
	CampaignKeyword string `json:"_rck,omitempty"`

	// Action info
	PageCustomVars string `json:"cvar,omitempty"`
	Outlink        string `json:"link,omitempty"`
	Download       string `json:"download,omitempty"`
	SearchKeyword  string `json:"search,omitempty"`
	SearchCategory string `json:"search_cat,omitempty"`
	SearchCount    string `json:"search_count,omitempty"`
	PageViewID     string `json:"pv_id,omitempty"`
	GoalID         string `json:"idgoal,omitempty"`
	Revenue        string `json:"revenue,omitempty"`
	Charset        string `json:"cs,omitempty"`
	CustomAction   string `json:"ca,omitempty"`

	// Page Performance
	PerfNetwork     string `json:"pf_net,omitempty"`
	PerfServer      string `json:"pf_srv,omitempty"`
	PerfTransfer    string `json:"pf_tfr,omitempty"`
	PerfDOMProcessing string `json:"pf_dm1,omitempty"`
	PerfDOMCompletion string `json:"pf_dm2,omitempty"`
	PerfOnLoad      string `json:"pf_onl,omitempty"`

	// Event Tracking
	EventCategory string `json:"e_c,omitempty"`
	EventAction   string `json:"e_a,omitempty"`
	EventName     string `json:"e_n,omitempty"`
	EventValue    string `json:"e_v,omitempty"`

	// Content Tracking
	ContentName       string `json:"c_n,omitempty"`
	ContentPiece      string `json:"c_p,omitempty"`
	ContentTarget     string `json:"c_t,omitempty"`
	ContentInteraction string `json:"c_i,omitempty"`

	// Ecommerce
	EcommerceOrderID string `json:"ec_id,omitempty"`
	EcommerceItems   string `json:"ec_items,omitempty"`
	EcommerceSubtotal string `json:"ec_st,omitempty"`
	EcommerceTax     string `json:"ec_tx,omitempty"`
	EcommerceShipping string `json:"ec_sh,omitempty"`
	EcommerceDiscount string `json:"ec_dt,omitempty"`
	ProductCategory  string `json:"_pkc,omitempty"`
	ProductPrice     string `json:"_pkp,omitempty"`
	ProductSKU       string `json:"_pks,omitempty"`
	ProductName      string `json:"_pkn,omitempty"`

	// Custom Dimensions (visit scope)
	VisitDimensions map[string]string `json:"visit_dimensions,omitempty"`
	// Custom Dimensions (action scope)
	ActionDimensions map[string]string `json:"action_dimensions,omitempty"`

	// Response control
	SendImage string `json:"send_image,omitempty"`
	Ping      string `json:"ping,omitempty"`

	// Bot tracking
	RecMode    string `json:"recMode,omitempty"`
	Bots       string `json:"bots,omitempty"`
	HTTPStatus string `json:"http_status,omitempty"`
	BytesTransferred string `json:"bw_bytes,omitempty"`
	BotSource  string `json:"source,omitempty"`

	// Auth/override (requires token_auth)
	TokenAuth    string `json:"token_auth,omitempty"`
	OverrideIP   string `json:"cip,omitempty"`
	OverrideTime string `json:"cdt,omitempty"`
	Country      string `json:"country,omitempty"`
	Region       string `json:"region,omitempty"`
	City         string `json:"city,omitempty"`
	Latitude     string `json:"lat,omitempty"`
	Longitude    string `json:"long,omitempty"`

	// Debug
	Debug string `json:"debug,omitempty"`

	// Bulk tracking
	BulkRequests []string `json:"requests,omitempty"`
}

// ParseEvent validates and parses tracking parameters into an Event
func ParseEvent(params map[string]string) (*Event, error) {
	siteID, ok := params["idsite"]
	if !ok || siteID == "" {
		return nil, errors.New("missing required parameter: idsite")
	}

	rec, ok := params["rec"]
	if !ok || rec == "" {
		return nil, errors.New("missing required parameter: rec")
	}

	e := &Event{
		SiteID:      siteID,
		Rec:         rec,
		ActionName:  params["action_name"],
		URL:         params["url"],
		VisitorID:   params["_id"],
		Rand:        params["rand"],
		APIVersion:  params["apiv"],
		Referrer:    params["urlref"],
		Resolution:  params["res"],
		Hour:        params["h"],
		Minute:      params["m"],
		Second:      params["s"],
		UserAgent:   params["ua"],
		ClientHints: params["uadata"],
		Language:    params["lang"],
		UserID:      params["uid"],
		VisitorUUID: params["cid"],
		NewVisit:    params["new_visit"],
		CustomVars:  params["_cvar"],
		Cookie:      params["cookie"],
		CampaignName:    params["_rcn"],
		CampaignKeyword: params["_rck"],
		PageCustomVars:  params["cvar"],
		Outlink:         params["link"],
		Download:        params["download"],
		SearchKeyword:   params["search"],
		SearchCategory:  params["search_cat"],
		SearchCount:     params["search_count"],
		PageViewID:      params["pv_id"],
		GoalID:          params["idgoal"],
		Revenue:         params["revenue"],
		Charset:         params["cs"],
		CustomAction:    params["ca"],
		PerfNetwork:     params["pf_net"],
		PerfServer:      params["pf_srv"],
		PerfTransfer:    params["pf_tfr"],
		PerfDOMProcessing: params["pf_dm1"],
		PerfDOMCompletion: params["pf_dm2"],
		PerfOnLoad:      params["pf_onl"],
		EventCategory:   params["e_c"],
		EventAction:     params["e_a"],
		EventName:       params["e_n"],
		EventValue:      params["e_v"],
		ContentName:     params["c_n"],
		ContentPiece:    params["c_p"],
		ContentTarget:   params["c_t"],
		ContentInteraction: params["c_i"],
		EcommerceOrderID:  params["ec_id"],
		EcommerceItems:    params["ec_items"],
		EcommerceSubtotal: params["ec_st"],
		EcommerceTax:      params["ec_tx"],
		EcommerceShipping: params["ec_sh"],
		EcommerceDiscount: params["ec_dt"],
		ProductCategory:   params["_pkc"],
		ProductPrice:      params["_pkp"],
		ProductSKU:        params["_pks"],
		ProductName:       params["_pkn"],
		SendImage:       params["send_image"],
		Ping:            params["ping"],
		RecMode:         params["recMode"],
		Bots:            params["bots"],
		HTTPStatus:      params["http_status"],
		BytesTransferred: params["bw_bytes"],
		BotSource:       params["source"],
		TokenAuth:       params["token_auth"],
		OverrideIP:      params["cip"],
		OverrideTime:    params["cdt"],
		Country:         params["country"],
		Region:          params["region"],
		City:            params["city"],
		Latitude:        params["lat"],
		Longitude:       params["long"],
		Debug:           params["debug"],
	}

	// Parse plugin flags
	e.Plugins = make(map[string]string)
	for _, plugin := range []string{"fla", "java", "dir", "qt", "realp", "pdf", "wma", "gears", "ag"} {
		if v := params[plugin]; v != "" {
			e.Plugins[plugin] = v
		}
	}

	// Parse custom dimensions (visit scope: dimension1-dimension999)
	e.VisitDimensions = make(map[string]string)
	e.ActionDimensions = make(map[string]string)
	for key, val := range params {
		if strings.HasPrefix(key, "dimension") {
			// Check if it's action scope by checking if a "ca" flag is set
			// or default to visit scope
			if e.CustomAction == "1" {
				e.ActionDimensions[key] = val
			} else {
				e.VisitDimensions[key] = val
			}
		}
	}

	return e, nil
}
```

- [ ] **Step 4: Run all tracker tests to verify pass**

```bash
go test ./internal/tracker/ -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tracker/event.go internal/tracker/event_test.go
git commit -m "feat: extend Event struct with full Matomo parameter set"
```

---

## Chunk 2: Response Format — send_image and ping

**Goal:** Handle `send_image=0` (HTTP 204 response) and `ping=1` (heartbeat requests that don't track new visits/actions).

**Files:**
- Modify: `internal/api/handler.go`
- Modify: `internal/api/handler_test.go`

### Task 2.1: Handle send_image=0 (HTTP 204 No Content)

- [ ] **Step 1: Write failing test for send_image=0**

Add to `internal/api/handler_test.go`:

```go
func TestTrackHandler_SendImageZero_Returns204(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1&send_image=0", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, rr.Code)
	}

	if rr.Body.Len() != 0 {
		t.Errorf("expected empty body for 204, got %d bytes", rr.Body.Len())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/api/ -v -run TestTrackHandler_SendImageZero_Returns204
```
Expected: FAIL — returns 200 with GIF

- [ ] **Step 3: Modify TrackHandler to check send_image**

In `internal/api/handler.go`, add before writing GIF:

```go
// Check if client wants HTTP 204 instead of GIF
if event.SendImage == "0" {
	w.WriteHeader(http.StatusNoContent)
	return
}
```

- [ ] **Step 4: Run test to verify pass**

```bash
go test ./internal/api/ -v -run TestTrackHandler_SendImageZero_Returns204
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler.go internal/api/handler_test.go
git commit -m "feat: support send_image=0 for HTTP 204 response"
```

### Task 2.2: Handle ping=1 (heartbeat)

- [ ] **Step 6: Write failing test for ping=1**

Add to `internal/api/handler_test.go`:

```go
func TestTrackHandler_Ping_Returns204(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1&ping=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	// Heartbeat requests should return 204 (no new action)
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status %d for ping, got %d", http.StatusNoContent, rr.Code)
	}
}
```

- [ ] **Step 7: Run test to verify it fails**

```bash
go test ./internal/api/ -v -run TestTrackHandler_Ping_Returns204
```
Expected: FAIL

- [ ] **Step 8: Add ping handling**

In `internal/api/handler.go`, check for `ping=1` and return 204:

```go
// Heartbeat request — update visit duration but don't track new action
if event.Ping == "1" {
	// TODO: In a real implementation, update visit's last action time
	// For now, just acknowledge with 204
	w.WriteHeader(http.StatusNoContent)
	return
}
```

- [ ] **Step 9: Run test to verify pass**

```bash
go test ./internal/api/ -v -run TestTrackHandler_Ping_Returns204
```
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add internal/api/handler.go internal/api/handler_test.go
git commit -m "feat: support ping=1 heartbeat requests"
```

---

## Chunk 3: Action Type Detection

**Goal:** Detect what type of action is being tracked (pageview, event, goal, outlink, download, search, ecommerce, content) and set a type field for NATS routing.

**Files:**
- Modify: `internal/tracker/event.go`
- Modify: `internal/tracker/event_test.go`
- Modify: `internal/nats/publisher.go`

### Task 3.1: Add ActionType field to Event

- [ ] **Step 1: Add ActionType field and detection logic**

Add to `Event` struct:
```go
ActionType string `json:"action_type,omitempty"`
```

Add detection function to `internal/tracker/event.go`:

```go
// detectActionType determines what kind of action is being tracked
func detectActionType(params map[string]string) string {
	switch {
	case params["ping"] == "1":
		return "heartbeat"
	case params["e_c"] != "" && params["e_a"] != "":
		return "event"
	case params["idgoal"] != "" && params["idgoal"] != "0":
		return "goal"
	case params["idgoal"] == "0":
		return "ecommerce"
	case params["search"] != "":
		return "search"
	case params["link"] != "":
		return "outlink"
	case params["download"] != "":
		return "download"
	case params["c_n"] != "" && params["c_i"] != "":
		return "content_interaction"
	case params["c_n"] != "":
		return "content_impression"
	default:
		return "pageview"
	}
}
```

Call it in `ParseEvent`:
```go	e.ActionType = detectActionType(params)
```

- [ ] **Step 2: Write tests for action type detection**

Add to `internal/tracker/event_test.go`:

```go
func TestDetectActionType(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]string
		expected string
	}{
		{"pageview", map[string]string{"idsite": "1", "rec": "1"}, "pageview"},
		{"event", map[string]string{"idsite": "1", "rec": "1", "e_c": "Video", "e_a": "Play"}, "event"},
		{"goal", map[string]string{"idsite": "1", "rec": "1", "idgoal": "2"}, "goal"},
		{"ecommerce", map[string]string{"idsite": "1", "rec": "1", "idgoal": "0"}, "ecommerce"},
		{"search", map[string]string{"idsite": "1", "rec": "1", "search": "query"}, "search"},
		{"outlink", map[string]string{"idsite": "1", "rec": "1", "link": "https://out.com"}, "outlink"},
		{"download", map[string]string{"idsite": "1", "rec": "1", "download": "https://ex.com/f.pdf"}, "download"},
		{"heartbeat", map[string]string{"idsite": "1", "rec": "1", "ping": "1"}, "heartbeat"},
		{"content_impression", map[string]string{"idsite": "1", "rec": "1", "c_n": "Ad"}, "content_impression"},
		{"content_interaction", map[string]string{"idsite": "1", "rec": "1", "c_n": "Ad", "c_i": "click"}, "content_interaction"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _ := ParseEvent(tt.params)
			if event.ActionType != tt.expected {
				t.Errorf("expected action type %s, got %s", tt.expected, event.ActionType)
			}
		})
	}
}
```

- [ ] **Step 3: Update NATS subject to use action type**

Modify `internal/nats/publisher.go`:

```go
subject := fmt.Sprintf("tracker.%s.%s", event.SiteID, event.ActionType)
```

- [ ] **Step 4: Update publisher tests**

Change test assertions from `tracker.1.pageview` to `tracker.1.pageview` (should still work for default).
Add test for event type:

```go
func TestPublisher_PublishEvent_EventType(t *testing.T) {
	mock := &MockPublisher{}
	publisher := NewPublisher(mock)

	event := &tracker.Event{
		SiteID:        "1",
		ActionType:    "event",
		EventCategory: "Video",
		EventAction:   "Play",
	}

	_ = publisher.PublishEvent(event)

	if mock.Subject != "tracker.1.event" {
		t.Errorf("expected subject tracker.1.event, got %s", mock.Subject)
	}
}
```

- [ ] **Step 5: Run all tests**

```bash
go test ./... -race
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tracker/event.go internal/tracker/event_test.go internal/nats/publisher.go internal/nats/publisher_test.go
git commit -m "feat: detect action type for NATS subject routing"
```

---

## Chunk 4: Bulk Tracking (JSON POST)

**Goal:** Support Matomo bulk tracking via JSON POST body with `requests` array.

**Files:**
- Create: `internal/tracker/bulk.go`
- Create: `internal/tracker/bulk_test.go`
- Modify: `internal/api/handler.go`
- Modify: `internal/api/handler_test.go`

### Task 4.1: Parse bulk tracking requests

- [ ] **Step 1: Create bulk parser**

Create `internal/tracker/bulk.go`:

```go
package tracker

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// BulkRequest represents a bulk tracking payload
type BulkRequest struct {
	Requests  []string `json:"requests"`
	TokenAuth string   `json:"token_auth,omitempty"`
}

// ParseBulkRequest parses a JSON bulk tracking request
func ParseBulkRequest(body []byte) (*BulkRequest, error) {
	var bulk BulkRequest
	if err := json.Unmarshal(body, &bulk); err != nil {
		return nil, fmt.Errorf("invalid bulk request JSON: %w", err)
	}

	if len(bulk.Requests) == 0 {
		return nil, fmt.Errorf("bulk request must contain at least one request")
	}

	return &bulk, nil
}

// ExtractParamsFromQueryString parses a single request string (e.g., "?idsite=1&rec=1")
func ExtractParamsFromQueryString(queryString string) (map[string]string, error) {
	// Remove leading ? if present
	queryString = strings.TrimPrefix(queryString, "?")

	values, err := url.ParseQuery(queryString)
	if err != nil {
		return nil, fmt.Errorf("invalid query string: %w", err)
	}

	params := make(map[string]string)
	for key, vals := range values {
		if len(vals) > 0 {
			params[key] = vals[0]
		}
	}

	return params, nil
}
```

- [ ] **Step 2: Create bulk parser tests**

Create `internal/tracker/bulk_test.go`:

```go
package tracker

import (
	"testing"
)

func TestParseBulkRequest(t *testing.T) {
	jsonBody := `{
		"requests": [
			"?idsite=1&rec=1&url=https://example.com",
			"?idsite=1&rec=1&url=https://example.net"
		],
		"token_auth": "abc123"
	}`

	bulk, err := ParseBulkRequest([]byte(jsonBody))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bulk.Requests) != 2 {
		t.Errorf("expected 2 requests, got %d", len(bulk.Requests))
	}
	if bulk.TokenAuth != "abc123" {
		t.Errorf("expected token_auth abc123, got %s", bulk.TokenAuth)
	}
}

func TestParseBulkRequest_Empty(t *testing.T) {
	jsonBody := `{"requests": []}`
	_, err := ParseBulkRequest([]byte(jsonBody))
	if err == nil {
		t.Fatal("expected error for empty requests")
	}
}

func TestExtractParamsFromQueryString(t *testing.T) {
	params, err := ExtractParamsFromQueryString("?idsite=1&rec=1&action_name=Test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params["idsite"] != "1" {
		t.Errorf("expected idsite 1, got %s", params["idsite"])
	}
	if params["action_name"] != "Test" {
		t.Errorf("expected action_name Test, got %s", params["action_name"])
	}
}
```

- [ ] **Step 3: Modify handler to support bulk tracking**

In `internal/api/handler.go`, add bulk detection at the start of the handler:

```go
// Check for bulk tracking (JSON POST body with "requests" array)
if r.Method == http.MethodPost && r.Header.Get("Content-Type") == "application/json" {
	body := make([]byte, r.ContentLength)
	if _, err := r.Body.Read(body); err == nil {
		if bulk, err := tracker.ParseBulkRequest(body); err == nil {
			// Process bulk requests
			for _, reqStr := range bulk.Requests {
				params, err := tracker.ExtractParamsFromQueryString(reqStr)
				if err != nil {
					continue
				}
				if bulk.TokenAuth != "" {
					params["token_auth"] = bulk.TokenAuth
				}
				processEvent(w, r, publisher, params)
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}
}
```

Extract single-event processing into a helper:

```go
func processEvent(w http.ResponseWriter, r *http.Request, publisher EventPublisher, params map[string]string) {
	event, err := tracker.ParseEvent(params)
	if err != nil {
		return // Bulk processing ignores individual errors
	}

	if publisher != nil {
		_ = publisher.PublishEvent(event)
	}
}
```

- [ ] **Step 4: Add bulk tracking test**

Add to `internal/api/handler_test.go`:

```go
func TestTrackHandler_BulkTracking(t *testing.T) {
	mock := &MockPublisher{}
	body := `{"requests":["?idsite=1&rec=1&url=https://example.com","?idsite=1&rec=1&url=https://example.net"],"token_auth":"abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/track", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler := TrackHandler(mock)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if len(mock.Events) != 2 {
		t.Errorf("expected 2 published events, got %d", len(mock.Events))
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./... -race
```

- [ ] **Step 6: Commit**

```bash
git add internal/tracker/bulk.go internal/tracker/bulk_test.go internal/api/handler.go internal/api/handler_test.go
git commit -m "feat: support bulk tracking via JSON POST"
```

---

## Chunk 5: Debug Mode

**Goal:** Support `debug=1` query parameter that returns debug info instead of GIF.

**Files:**
- Modify: `internal/api/handler.go`
- Modify: `internal/api/handler_test.go`

### Task 5.1: Implement debug mode

- [ ] **Step 1: Write failing test**

```go
func TestTrackHandler_Debug_ReturnsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1&debug=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected JSON content type, got %s", contentType)
	}

	var debugResp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &debugResp); err != nil {
		t.Fatalf("expected valid JSON, got: %s", rr.Body.String())
	}
}
```

- [ ] **Step 2: Add debug response**

In `internal/api/handler.go`, after event parsing:

```go
if event.Debug == "1" {
	w.Header().Set("Content-Type", "application/json")
	debugInfo := map[string]interface{}{
		"debug":       true,
		"idsite":      event.SiteID,
		"action_type": event.ActionType,
		"parsed_params": params,
	}
	json.NewEncoder(w).Encode(debugInfo)
	return
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/api/ -v -run TestTrackHandler_Debug_ReturnsJSON
```

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler.go internal/api/handler_test.go
git commit -m "feat: add debug=1 mode for troubleshooting"
```

---

## Chunk 6: Web Test Page Update

**Goal:** Update `web/test.html` to test all tracking types.

**Files:**
- Modify: `web/test.html`

- [ ] **Step 1: Update test page with all tracking types**

Replace `web/test.html` with a comprehensive test page that tests pageviews, events, goals, searches, outlinks, downloads, and ecommerce.

- [ ] **Step 2: Commit**

```bash
git add web/test.html
git commit -m "feat: update test page with all Matomo tracking types"
```

---

## Chunk 7: Final Verification

**Goal:** Ensure all tests pass and coverage remains >90%.

- [ ] **Step 1: Run full test suite**

```bash
go test ./... -race -coverprofile=coverage.out
```

- [ ] **Step 2: Check coverage**

```bash
go tool cover -func=coverage.out | tail -1
```
Expected: >90%

- [ ] **Step 3: Run benchmarks**

```bash
go test ./internal/api/ -bench=.
go test ./internal/nats/ -bench=.
go test ./internal/tracker/ -bench=.
```

- [ ] **Step 4: End-to-end test with NATS**

```bash
nats-server -js &
go run ./cmd/tracker &
# Test pageviews, events, goals, searches
curl "http://localhost:8080/track?idsite=1&rec=1&action_name=Home&url=https://example.com"
curl "http://localhost:8080/track?idsite=1&rec=1&e_c=Video&e_a=Play&e_n=Intro"
nats sub "tracker.1.pageview"
nats sub "tracker.1.event"
```

- [ ] **Step 5: Final commit and PR**

```bash
git add -A
git commit -m "feat: full Matomo Tracking API compatibility

- All query parameters supported (50+ fields)
- Action type detection: pageview, event, goal, outlink, download, search, ecommerce, content, heartbeat
- send_image=0 returns HTTP 204
- ping=1 heartbeat support
- Bulk tracking via JSON POST
- debug=1 mode
- NATS subjects: tracker.{site_id}.{action_type}"
```

---

## Parameter Coverage Checklist

| Parameter | Status | Notes |
|-----------|--------|-------|
| `idsite` | ✅ | Required |
| `rec` | ✅ | Required |
| `action_name` | ✅ | Page title |
| `url` | ✅ | Page URL |
| `_id` | ✅ | Visitor ID |
| `rand` | ✅ | Cache buster |
| `apiv` | ✅ | API version |
| `urlref` | ✅ | Referrer |
| `res` | ✅ | Screen resolution |
| `h`, `m`, `s` | ✅ | Local time |
| `ua` | ✅ | User agent override |
| `uadata` | ✅ | Client hints |
| `lang` | ✅ | Language |
| `uid` | ✅ | User ID |
| `cid` | ✅ | Visitor UUID |
| `new_visit` | ✅ | Force new visit |
| `cookie` | ✅ | Cookie support |
| `fla`, `java`, etc. | ✅ | Plugin flags |
| `_rcn`, `_rck` | ✅ | Campaign attribution |
| `cvar` | ✅ | Page custom vars |
| `_cvar` | ✅ | Visit custom vars |
| `link` | ✅ | Outlink tracking |
| `download` | ✅ | Download tracking |
| `search`, `search_cat`, `search_count` | ✅ | Site search |
| `pv_id` | ✅ | Pageview ID |
| `idgoal` | ✅ | Goal tracking |
| `revenue` | ✅ | Goal revenue |
| `cs` | ✅ | Charset |
| `ca` | ✅ | Custom action flag |
| `pf_*` | ✅ | Page performance |
| `e_c`, `e_a`, `e_n`, `e_v` | ✅ | Event tracking |
| `c_n`, `c_p`, `c_t`, `c_i` | ✅ | Content tracking |
| `ec_*`, `_pk*` | ✅ | Ecommerce |
| `dimension[0-999]` | ✅ | Custom dimensions |
| `send_image` | ✅ | HTTP 204 response |
| `ping` | ✅ | Heartbeat |
| `recMode`, `bots` | ✅ | Bot tracking |
| `token_auth` | ✅ | Auth |
| `cip`, `cdt` | ✅ | Override IP/time |
| `country`, `region`, `city`, `lat`, `long` | ✅ | Geo override |
| `debug` | ✅ | Debug mode |
| `requests` (bulk) | ✅ | Bulk tracking |
| `queuedtracking` | ⏳ | Not applicable (no queue) |
| `ma_*` (media) | ⏳ | Premium plugin |
| `cra_*` (crash) | ⏳ | Premium plugin |

---

## Compatibility Matrix

| SDK/Client | Compatible | Notes |
|------------|-----------|-------|
| Matomo JS Tracker | ✅ | Full parameter support, bulk tracking, ping |
| Matomo PHP Tracker | ✅ | All parameters, bulk tracking |
| Matomo Java Tracker | ✅ | All parameters, bulk tracking |
| Matomo iOS SDK | ✅ | All mobile parameters |
| Matomo Android SDK | ✅ | All mobile parameters |
| Log Importer | ✅ | Bulk tracking support |
