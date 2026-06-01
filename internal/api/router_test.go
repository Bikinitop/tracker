package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter_TrackEndpoint_Branded(t *testing.T) {
	router := NewRouter(&noopPublisher{})

	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestRouter_TrackEndpoint_MatomoAlias(t *testing.T) {
	router := NewRouter(&noopPublisher{})

	req := httptest.NewRequest(http.MethodGet, "/matomo.php?idsite=1&rec=1", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestRouter_HealthEndpoint(t *testing.T) {
	router := NewRouter(&noopPublisher{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if rr.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %s", rr.Body.String())
	}
}

func TestRouter_UnknownEndpoint(t *testing.T) {
	router := NewRouter(&noopPublisher{})

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
