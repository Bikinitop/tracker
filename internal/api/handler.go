package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/bikinitop/tracker/internal/tracker"
)

// 1x1 transparent GIF pixel (43 bytes)
var pixelGIF = []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0x21, 0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44, 0x01, 0x00, 0x3b}

// EventPublisher defines the interface for publishing tracking events
type EventPublisher interface {
	PublishEvent(event *tracker.Event) error
}

// TrackHandler returns an HTTP handler for Matomo-compatible tracking requests
func TrackHandler(publisher EventPublisher) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Check for bulk tracking (JSON POST body with "requests" array)
		if r.Method == http.MethodPost && strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read request body", http.StatusBadRequest)
				return
			}
			bulk, err := tracker.ParseBulkRequest(body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			processBulkRequests(w, publisher, bulk)
			return
		}

		params := make(map[string]string)

		// Parse query parameters (always present)
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				params[key] = values[0]
			}
		}

		// Parse POST form body if present
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err == nil {
				for key, values := range r.Form {
					if len(values) > 0 {
						params[key] = values[0]
					}
				}
			}
		}

		processSingleRequest(w, publisher, params)
	})
}

func processBulkRequests(w http.ResponseWriter, publisher EventPublisher, bulk *tracker.BulkRequest) {
	successCount := 0
	for _, reqStr := range bulk.Requests {
		params, err := tracker.ExtractParamsFromQueryString(reqStr)
		if err != nil {
			continue
		}
		if bulk.TokenAuth != "" {
			params["token_auth"] = bulk.TokenAuth
		}
		if publishEvent(publisher, params) {
			successCount++
		}
	}
	if successCount == 0 {
		http.Error(w, "all bulk requests failed", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func processSingleRequest(w http.ResponseWriter, publisher EventPublisher, params map[string]string) {
	event, err := tracker.ParseEvent(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if publisher != nil {
		if err := publisher.PublishEvent(event); err != nil {
			http.Error(w, "failed to publish event", http.StatusInternalServerError)
			return
		}
	}

	// Debug mode returns JSON instead of GIF
	if event.Debug == "1" {
		w.Header().Set("Content-Type", "application/json")
		debugInfo := map[string]interface{}{
			"debug":         true,
			"idsite":        event.SiteID,
			"action_type":   event.ActionType,
			"parsed_params": params,
		}
		json.NewEncoder(w).Encode(debugInfo)
		return
	}

	// Heartbeat request — update visit duration but don't track new action
	if event.Ping == "1" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Client requests HTTP 204 instead of GIF image (Chrome Apps, etc.)
	if event.SendImage == "0" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	if _, err := w.Write(pixelGIF); err != nil {
		// Network errors are expected; log but don't fail
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}

// publishEvent attempts to publish a tracking event. Returns true on success.
func publishEvent(publisher EventPublisher, params map[string]string) bool {
	event, err := tracker.ParseEvent(params)
	if err != nil {
		return false
	}
	if publisher != nil {
		if err := publisher.PublishEvent(event); err != nil {
			return false
		}
	}
	return true
}
