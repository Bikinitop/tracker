package api

import (
	"net/http"

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
		params := make(map[string]string)
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				params[key] = values[0]
			}
		}

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

		w.Header().Set("Content-Type", "image/gif")
		w.Write(pixelGIF)
	})
}
