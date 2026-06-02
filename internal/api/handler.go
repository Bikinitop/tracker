package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bikinitop/tracker/internal/circuitbreaker"
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

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
	var errors []string
	for i, reqStr := range bulk.Requests {
		params, err := tracker.ExtractParamsFromQueryString(reqStr)
		if err != nil {
			errors = append(errors, fmt.Sprintf("request[%d]: %v", i, err))
			continue
		}
		if bulk.TokenAuth != "" {
			params["token_auth"] = bulk.TokenAuth
		}
		if err := publishEvent(publisher, params); err != nil {
			errors = append(errors, fmt.Sprintf("request[%d]: %v", i, err))
			continue
		}
		successCount++
	}

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"status":  "success",
		"tracked": successCount,
		"failed":  len(bulk.Requests) - successCount,
	}
	if len(errors) > 0 {
		resp["errors"] = errors
	}
	json.NewEncoder(w).Encode(resp)
}

func processSingleRequest(w http.ResponseWriter, publisher EventPublisher, params map[string]string) {
	event, err := tracker.ParseEvent(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if publisher != nil {
		if err := publisher.PublishEvent(event); err != nil {
			if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
				http.Error(w, "service unavailable", http.StatusServiceUnavailable)
				return
			}
			http.Error(w, "failed to publish event", http.StatusInternalServerError)
			return
		}
	}

	// Debug mode returns JSON instead of GIF
	if event.Debug == "1" {
		w.Header().Set("Content-Type", "application/json")
		debugInfo := map[string]interface{}{
			"debug":       true,
			"idsite":      event.SiteID,
			"action_type": event.ActionType,
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
	w.Write(pixelGIF)
}

// publishEvent attempts to publish a tracking event. Returns nil on success.
func publishEvent(publisher EventPublisher, params map[string]string) error {
	event, err := tracker.ParseEvent(params)
	if err != nil {
		return err
	}
	if publisher != nil {
		if err := publisher.PublishEvent(event); err != nil {
			return err
		}
	}
	return nil
}
