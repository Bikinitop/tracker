package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	req := httptest.NewRequest(http.MethodGet, "/matomo.php?idsite=1&rec=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestTrackHandler_MissingIDSite_ReturnsBadRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/matomo.php?rec=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTrackHandler_MissingRec_ReturnsBadRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/matomo.php?idsite=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTrackHandler_ReturnsPixelGIF(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/matomo.php?idsite=1&rec=1", nil)
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

func TestTrackHandler_PublishesEvent(t *testing.T) {
	mock := &MockPublisher{}
	req := httptest.NewRequest(http.MethodGet, "/matomo.php?idsite=1&rec=1&url=https%3A%2F%2Fexample.com&action_name=Test", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/matomo.php?idsite=1&rec=1", nil)
	rr := httptest.NewRecorder()

	handler := TrackHandler(failingPublisher)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

type FailingPublisher struct{}

func (f *FailingPublisher) PublishEvent(event *tracker.Event) error {
	return json.Unmarshal([]byte("invalid"), &struct{}{})
}
