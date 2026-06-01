package tracker

import (
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
		"_id":     "abc123",
		"uid":     "user456",
		"cid":     "uuid789",
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
	if event.VisitorID != "abc123" {
		t.Errorf("expected VisitorID abc123, got %s", event.VisitorID)
	}
	if event.UserID != "user456" {
		t.Errorf("expected UserID user456, got %s", event.UserID)
	}
	if event.VisitorUUID != "uuid789" {
		t.Errorf("expected VisitorUUID uuid789, got %s", event.VisitorUUID)
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
