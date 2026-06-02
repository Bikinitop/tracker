package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bikinitop/tracker/internal/tracker"
)

// MockPublisher implements NATS publishing for tests
type MockPublisher struct {
	Events []*tracker.Event
}

func (m *MockPublisher) PublishEvent(event *tracker.Event) error {
	m.Events = append(m.Events, event)
	return nil
}

func TestTrackHandler_ReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestTrackHandler_MissingIDSite_ReturnsBadRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/track?rec=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTrackHandler_MissingRec_ReturnsBadRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTrackHandler_ReturnsPixelGIF(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	expected := []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0x21, 0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44, 0x01, 0x00, 0x3b}
	if !bytes.Equal(rr.Body.Bytes(), expected) {
		t.Errorf("expected 1x1 GIF pixel, got %v", rr.Body.Bytes())
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "image/gif" {
		t.Errorf("expected Content-Type image/gif, got %s", contentType)
	}
}

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

func TestTrackHandler_Ping_Returns204(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1&ping=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status %d for ping, got %d", http.StatusNoContent, rr.Code)
	}
}

func TestTrackHandler_ReturnsCORSHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin *, got %s", origin)
	}

	methods := rr.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "GET") || !strings.Contains(methods, "POST") {
		t.Errorf("expected CORS methods to include GET and POST, got %s", methods)
	}

	headers := rr.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "Content-Type") {
		t.Errorf("expected Access-Control-Allow-Headers to include Content-Type, got %s", headers)
	}
}

func TestTrackHandler_ReturnsCacheControl(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	cacheControl := rr.Header().Get("Cache-Control")
	if !strings.Contains(cacheControl, "no-cache") {
		t.Errorf("expected Cache-Control to contain no-cache, got %s", cacheControl)
	}

	pragma := rr.Header().Get("Pragma")
	if pragma != "no-cache" {
		t.Errorf("expected Pragma no-cache, got %s", pragma)
	}
}

func TestTrackHandler_OptionsRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/track", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d for OPTIONS, got %d", http.StatusOK, rr.Code)
	}
}

func TestTrackHandler_POSTBodyParams(t *testing.T) {
	mock := &MockPublisher{}
	body := "idsite=1&rec=1&url=https%3A%2F%2Fexample.com&action_name=PostTest"
	req := httptest.NewRequest(http.MethodPost, "/track", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler := TrackHandler(mock)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if len(mock.Events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(mock.Events))
	}

	event := mock.Events[0]
	if event.SiteID != "1" {
		t.Errorf("expected SiteID 1, got %s", event.SiteID)
	}
	if event.URL != "https://example.com" {
		t.Errorf("expected URL https://example.com, got %s", event.URL)
	}
	if event.ActionName != "PostTest" {
		t.Errorf("expected ActionName PostTest, got %s", event.ActionName)
	}
}

func TestTrackHandler_POSTOverridesQuery(t *testing.T) {
	mock := &MockPublisher{}
	body := "idsite=2&rec=1&url=https%3A%2F%2Fpost.com"
	req := httptest.NewRequest(http.MethodPost, "/track?idsite=1&rec=1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler := TrackHandler(mock)
	handler.ServeHTTP(rr, req)

	if len(mock.Events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(mock.Events))
	}

	event := mock.Events[0]
	if event.SiteID != "2" {
		t.Errorf("expected POST body to override query param, got SiteID %s", event.SiteID)
	}
	if event.URL != "https://post.com" {
		t.Errorf("expected URL https://post.com, got %s", event.URL)
	}
}

func TestTrackHandler_PublishesEvent(t *testing.T) {
	mock := &MockPublisher{}
	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1&url=https%3A%2F%2Fexample.com&action_name=Test", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(mock)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if len(mock.Events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(mock.Events))
	}

	event := mock.Events[0]
	if event.SiteID != "1" {
		t.Errorf("expected SiteID 1, got %s", event.SiteID)
	}
	if event.URL != "https://example.com" {
		t.Errorf("expected URL https://example.com, got %s", event.URL)
	}
	if event.ActionName != "Test" {
		t.Errorf("expected ActionName Test, got %s", event.ActionName)
	}
}

func TestTrackHandler_PublishError_ReturnsServerError(t *testing.T) {
	failingPublisher := &FailingPublisher{}
	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(failingPublisher)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

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

	// Check token_auth was passed through
	if mock.Events[0].TokenAuth != "abc123" {
		t.Errorf("expected token_auth abc123, got %s", mock.Events[0].TokenAuth)
	}

	// Verify JSON response structure
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got: %s", rr.Body.String())
	}
	if resp["status"] != "success" {
		t.Errorf("expected status 'success', got %v", resp["status"])
	}
	if resp["tracked"] != float64(2) {
		t.Errorf("expected tracked=2, got %v", resp["tracked"])
	}
	if resp["failed"] != float64(0) {
		t.Errorf("expected failed=0, got %v", resp["failed"])
	}
}

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

	if debugResp["debug"] != true {
		t.Errorf("expected debug=true")
	}
}

func TestTrackHandler_BulkTracking_PartialSuccess(t *testing.T) {
	body := `{"requests":["?idsite=1&rec=1","?invalid=1"]}`
	req := httptest.NewRequest(http.MethodPost, "/track", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got: %s", rr.Body.String())
	}
	if resp["tracked"] != float64(1) {
		t.Errorf("expected tracked=1, got %v", resp["tracked"])
	}
	if resp["failed"] != float64(1) {
		t.Errorf("expected failed=1, got %v", resp["failed"])
	}
	errs, ok := resp["errors"].([]interface{})
	if !ok || len(errs) != 1 {
		t.Errorf("expected 1 error in response, got %v", errs)
	}
}

func TestTrackHandler_BulkTracking_FullURL(t *testing.T) {
	body := `{"requests":["https://example.com/track?idsite=1&rec=1&action_name=FullURLTest"]}`
	req := httptest.NewRequest(http.MethodPost, "/track", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mock := &MockPublisher{}
	handler := TrackHandler(mock)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if len(mock.Events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(mock.Events))
	}
	if mock.Events[0].SiteID != "1" {
		t.Errorf("expected SiteID 1, got %s", mock.Events[0].SiteID)
	}
	if mock.Events[0].ActionName != "FullURLTest" {
		t.Errorf("expected ActionName FullURLTest, got %s", mock.Events[0].ActionName)
	}
}

func TestTrackHandler_BulkTracking_WithCharset(t *testing.T) {
	mock := &MockPublisher{}
	body := `{"requests":["?idsite=1&rec=1&url=https://example.com"]}`
	req := httptest.NewRequest(http.MethodPost, "/track", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	rr := httptest.NewRecorder()

	handler := TrackHandler(mock)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if len(mock.Events) != 1 {
		t.Errorf("expected 1 published event, got %d", len(mock.Events))
	}
}

func TestTrackHandler_BulkTracking_InvalidJSON_Returns400(t *testing.T) {
	body := `not json`
	req := httptest.NewRequest(http.MethodPost, "/track", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for invalid JSON, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTrackHandler_BulkTracking_AllInvalid_ReturnsJSON(t *testing.T) {
	body := `{"requests":["?invalid=1","?also=invalid"]}`
	req := httptest.NewRequest(http.MethodPost, "/track", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	// Returns 200 with tracked=0 and errors array
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got: %s", rr.Body.String())
	}
	if resp["tracked"] != float64(0) {
		t.Errorf("expected tracked=0, got %v", resp["tracked"])
	}
	if resp["failed"] != float64(2) {
		t.Errorf("expected failed=2, got %v", resp["failed"])
	}
	errs, ok := resp["errors"].([]interface{})
	if !ok || len(errs) == 0 {
		t.Errorf("expected errors array in response")
	}
}

type FailingPublisher struct{}

func (f *FailingPublisher) PublishEvent(event *tracker.Event) error {
	return json.Unmarshal([]byte("invalid"), &struct{}{})
}
