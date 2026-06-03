package tracker

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseEvent_ValidParams(t *testing.T) {
	params := map[string]string{
		"idsite":      "1",
		"rec":         "1",
		"url":         "https://example.com/page",
		"action_name": "Test Page",
	}

	event, err := ParseEvent(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.SiteID != "1" {
		t.Errorf("expected SiteID 1, got %s", event.SiteID)
	}
	if event.URL != "https://example.com/page" {
		t.Errorf("expected URL https://example.com/page, got %s", event.URL)
	}
	if event.ActionName != "Test Page" {
		t.Errorf("expected ActionName 'Test Page', got %s", event.ActionName)
	}
}

func TestParseEvent_MissingSiteID(t *testing.T) {
	params := map[string]string{
		"rec": "1",
	}

	_, err := ParseEvent(params)
	if err == nil {
		t.Fatal("expected error for missing idsite")
	}
}

func TestParseEvent_MissingRec(t *testing.T) {
	params := map[string]string{
		"idsite": "1",
	}

	_, err := ParseEvent(params)
	if err == nil {
		t.Fatal("expected error for missing rec")
	}
}

func TestParseEvent_EmptyParams(t *testing.T) {
	params := map[string]string{}

	_, err := ParseEvent(params)
	if err == nil {
		t.Fatal("expected error for empty params")
	}
}

func TestParseEvent_MinimalValid(t *testing.T) {
	params := map[string]string{
		"idsite": "1",
		"rec":    "1",
	}

	event, err := ParseEvent(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.SiteID != "1" {
		t.Errorf("expected SiteID 1, got %s", event.SiteID)
	}
	if event.URL != "" {
		t.Errorf("expected empty URL, got %s", event.URL)
	}
	if event.ActionName != "" {
		t.Errorf("expected empty ActionName, got %s", event.ActionName)
	}
}

func TestParseEvent_FullMatomoParams(t *testing.T) {
	params := map[string]string{
		"idsite":  "42",
		"rec":     "1",
		"url":     "https://example.com/product",
		"_id":     "abc123def4567890",
		"uid":     "user456",
		"cid":     "abc123def4567890",
		"res":     "1920x1080",
		"lang":    "en-US",
		"ua":      "Mozilla/5.0",
		"urlref":  "https://google.com",
	}

	event, err := ParseEvent(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.SiteID != "42" {
		t.Errorf("expected SiteID 42, got %s", event.SiteID)
	}
	if event.VisitorID != "abc123def4567890" {
		t.Errorf("expected VisitorID abc123def4567890, got %s", event.VisitorID)
	}
	if event.UserID != "user456" {
		t.Errorf("expected UserID user456, got %s", event.UserID)
	}
	if event.VisitorUUID != "abc123def4567890" {
		t.Errorf("expected VisitorUUID abc123def4567890, got %s", event.VisitorUUID)
	}
	if event.Resolution != "1920x1080" {
		t.Errorf("expected Resolution 1920x1080, got %s", event.Resolution)
	}
	if event.Language != "en-US" {
		t.Errorf("expected Language en-US, got %s", event.Language)
	}
	if event.UserAgent != "Mozilla/5.0" {
		t.Errorf("expected UserAgent Mozilla/5.0, got %s", event.UserAgent)
	}
	if event.Referrer != "https://google.com" {
		t.Errorf("expected Referrer https://google.com, got %s", event.Referrer)
	}
}

func TestParseEvent_AllMatomoFields(t *testing.T) {
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

	if event.SiteID != "1" {
		t.Errorf("SiteID mismatch, got %s", event.SiteID)
	}
	if event.ActionName != "Home" {
		t.Errorf("ActionName mismatch, got %s", event.ActionName)
	}
	if event.EventCategory != "Video" {
		t.Errorf("EventCategory mismatch, got %s", event.EventCategory)
	}
	if event.EventAction != "Play" {
		t.Errorf("EventAction mismatch, got %s", event.EventAction)
	}
	if event.EventName != "Intro" {
		t.Errorf("EventName mismatch, got %s", event.EventName)
	}
	if event.EventValue != "10.5" {
		t.Errorf("EventValue mismatch, got %s", event.EventValue)
	}
	if event.GoalID != "2" {
		t.Errorf("GoalID mismatch, got %s", event.GoalID)
	}
	if event.Revenue != "99.99" {
		t.Errorf("Revenue mismatch, got %s", event.Revenue)
	}
	if event.SearchKeyword != "query" {
		t.Errorf("SearchKeyword mismatch, got %s", event.SearchKeyword)
	}
	if event.SearchCategory != "Products" {
		t.Errorf("SearchCategory mismatch, got %s", event.SearchCategory)
	}
	if event.SearchCount != "42" {
		t.Errorf("SearchCount mismatch, got %s", event.SearchCount)
	}
	if event.Outlink != "https://outlink.com" {
		t.Errorf("Outlink mismatch, got %s", event.Outlink)
	}
	if event.Download != "https://example.com/file.pdf" {
		t.Errorf("Download mismatch, got %s", event.Download)
	}
	if event.PerfNetwork != "100" {
		t.Errorf("PerfNetwork mismatch, got %s", event.PerfNetwork)
	}
	if event.PerfServer != "200" {
		t.Errorf("PerfServer mismatch, got %s", event.PerfServer)
	}
	if event.PerfTransfer != "50" {
		t.Errorf("PerfTransfer mismatch, got %s", event.PerfTransfer)
	}
	if event.EcommerceOrderID != "ORDER-123" {
		t.Errorf("EcommerceOrderID mismatch, got %s", event.EcommerceOrderID)
	}
	if event.EcommerceSubtotal != "80.00" {
		t.Errorf("EcommerceSubtotal mismatch, got %s", event.EcommerceSubtotal)
	}
	if event.EcommerceTax != "5.00" {
		t.Errorf("EcommerceTax mismatch, got %s", event.EcommerceTax)
	}
	if event.EcommerceShipping != "10.00" {
		t.Errorf("EcommerceShipping mismatch, got %s", event.EcommerceShipping)
	}
	if event.EcommerceDiscount != "5.00" {
		t.Errorf("EcommerceDiscount mismatch, got %s", event.EcommerceDiscount)
	}
	if event.ContentName != "AdBanner" {
		t.Errorf("ContentName mismatch, got %s", event.ContentName)
	}
	if event.ContentPiece != "/img/ad.png" {
		t.Errorf("ContentPiece mismatch, got %s", event.ContentPiece)
	}
	if event.ContentTarget != "https://ad.com" {
		t.Errorf("ContentTarget mismatch, got %s", event.ContentTarget)
	}
	if event.ContentInteraction != "click" {
		t.Errorf("ContentInteraction mismatch, got %s", event.ContentInteraction)
	}
	if event.SendImage != "0" {
		t.Errorf("SendImage mismatch, got %s", event.SendImage)
	}
	if event.Ping != "1" {
		t.Errorf("Ping mismatch, got %s", event.Ping)
	}
	if event.RecMode != "2" {
		t.Errorf("RecMode mismatch, got %s", event.RecMode)
	}
	if event.Bots != "1" {
		t.Errorf("Bots mismatch, got %s", event.Bots)
	}
	if event.OverrideTime != "2024-01-15 10:30:00" {
		t.Errorf("OverrideTime mismatch, got %s", event.OverrideTime)
	}
	if event.Country != "us" {
		t.Errorf("Country mismatch, got %s", event.Country)
	}
	if event.City != "NYC" {
		t.Errorf("City mismatch, got %s", event.City)
	}
	if event.TokenAuth != "abc123" {
		t.Errorf("TokenAuth mismatch, got %s", event.TokenAuth)
	}
	if event.PageCustomVars != `{"1":["OS","Mac"]}` {
		t.Errorf("PageCustomVars mismatch, got %s", event.PageCustomVars)
	}
	if event.CustomVars != `{"2":["Browser","Chrome"]}` {
		t.Errorf("CustomVars mismatch, got %s", event.CustomVars)
	}
	if event.Hour != "14" {
		t.Errorf("Hour mismatch, got %s", event.Hour)
	}
	if event.Minute != "30" {
		t.Errorf("Minute mismatch, got %s", event.Minute)
	}
	if event.Second != "45" {
		t.Errorf("Second mismatch, got %s", event.Second)
	}
	if event.Rand != "123456" {
		t.Errorf("Rand mismatch, got %s", event.Rand)
	}
	if event.APIVersion != "1" {
		t.Errorf("APIVersion mismatch, got %s", event.APIVersion)
	}
	if event.EcommerceItems != `[["SKU1","Item1","Cat1",10.00,2]]` {
		t.Errorf("EcommerceItems mismatch, got %s", event.EcommerceItems)
	}
	if event.VisitDimensions["dimension1"] != "value1" {
		t.Errorf("VisitDimensions[dimension1] mismatch")
	}
	if event.ActionDimensions["dimension2"] != "" {
		// dimension2 should be in visit dimensions since ca is not set
		t.Errorf("ActionDimensions[dimension2] should be empty without ca=1")
	}
}

func TestParseEvent_CustomDimensionsActionScope(t *testing.T) {
	params := map[string]string{
		"idsite": "1", "rec": "1", "ca": "1",
		"dimension1": "action_value",
	}

	event, err := ParseEvent(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.ActionDimensions["dimension1"] != "action_value" {
		t.Errorf("expected dimension1 in action scope")
	}
	if event.VisitDimensions["dimension1"] != "" {
		t.Errorf("dimension1 should not be in visit scope when ca=1")
	}
}

func TestParseEvent_Plugins(t *testing.T) {
	params := map[string]string{
		"idsite": "1", "rec": "1",
		"fla": "1", "java": "1", "pdf": "1",
	}

	event, err := ParseEvent(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Plugins["fla"] != "1" {
		t.Errorf("expected flash plugin")
	}
	if event.Plugins["java"] != "1" {
		t.Errorf("expected java plugin")
	}
	if event.Plugins["pdf"] != "1" {
		t.Errorf("expected pdf plugin")
	}
	if event.Plugins["qt"] != "" {
		t.Errorf("qt should not be set")
	}
}

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

// Drift guard: each Event field populated from a single query param has a json
// tag equal to that param's key. Assert every such key is in handledParams, so a
// newly added input field that's forgotten in handledParams fails CI instead of
// silently double-publishing (once in its named field, once in Extra).
// Computed/aggregate fields (not read from a single param) are exempted.
func TestParseEvent_HandledParamsCoverAllMappedKeys(t *testing.T) {
	computed := map[string]struct{}{
		"action_type": {}, "visit_dimensions": {}, "action_dimensions": {},
		"plugins": {}, "extra": {}, "requests": {},
	}
	typ := reflect.TypeOf(Event{})
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		if _, exempt := computed[name]; exempt {
			continue
		}
		if _, ok := handledParams[name]; !ok {
			t.Errorf("Event.%s (json %q) is not in handledParams; it would double-publish into Extra",
				typ.Field(i).Name, name)
		}
	}
}

func TestParseEvent_DimensionKeysNotInExtra(t *testing.T) {
	e, err := ParseEvent(map[string]string{
		"idsite": "1", "rec": "1", "dimension1": "a", "dimension42": "b",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(e.Extra) != 0 {
		t.Errorf("dimensions must go to VisitDimensions, not Extra: %v", e.Extra)
	}
	if e.VisitDimensions["dimension1"] != "a" {
		t.Errorf("dimension1 not captured in VisitDimensions: %v", e.VisitDimensions)
	}
}

func TestParseEvent_PluginKeysNotInExtra(t *testing.T) {
	params := map[string]string{"idsite": "1", "rec": "1"}
	for _, k := range pluginKeys {
		params[k] = "1"
	}
	e, err := ParseEvent(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, k := range pluginKeys {
		if _, leaked := e.Extra[k]; leaked {
			t.Errorf("plugin flag %q leaked into Extra", k)
		}
		if e.Plugins[k] != "1" {
			t.Errorf("plugin flag %q not captured in Plugins", k)
		}
	}
}

func BenchmarkParseEvent(b *testing.B) {
	params := map[string]string{
		"idsite":      "1",
		"rec":         "1",
		"url":         "https://example.com/page",
		"action_name": "Test Page",
	}

	for i := 0; i < b.N; i++ {
		_, err := ParseEvent(params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

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

// Cross-product: each invalid kind (too short / too long / non-hex) demotes the
// right field under the right key while the OTHER field's valid ID survives —
// guarding the two hardcoded validateID call sites against a key/field mix-up
// or one bad ID clobbering the other.
func TestParseEvent_InvalidVisitorID_AllKindsBothFields(t *testing.T) {
	invalids := map[string]string{
		"tooShort": "abc",
		"tooLong":  "0123456789abcdef0", // 17 chars
		"nonHex":   "zzzzzzzzzzzzzzzz",
	}
	for name, bad := range invalids {
		t.Run(name, func(t *testing.T) {
			// _id invalid, cid valid → only _id demoted; cid survives lowercased.
			e, err := ParseEvent(map[string]string{
				"idsite": "1", "rec": "1", "_id": bad, "cid": "ABCDEF0123456789",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if e.VisitorID != "" || e.Extra["_id"] != bad {
				t.Errorf("_id: got VisitorID=%q Extra[_id]=%q, want \"\" and %q", e.VisitorID, e.Extra["_id"], bad)
			}
			if e.VisitorUUID != "abcdef0123456789" {
				t.Errorf("valid cid should survive lowercased, got %q", e.VisitorUUID)
			}
			if _, ok := e.Extra["cid"]; ok {
				t.Errorf("valid cid must not be in Extra")
			}

			// Symmetric: cid invalid, _id valid.
			e2, err := ParseEvent(map[string]string{
				"idsite": "1", "rec": "1", "_id": "0123456789ABCDEF", "cid": bad,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if e2.VisitorUUID != "" || e2.Extra["cid"] != bad {
				t.Errorf("cid: got VisitorUUID=%q Extra[cid]=%q, want \"\" and %q", e2.VisitorUUID, e2.Extra["cid"], bad)
			}
			if e2.VisitorID != "0123456789abcdef" {
				t.Errorf("valid _id should survive lowercased, got %q", e2.VisitorID)
			}
		})
	}
}
