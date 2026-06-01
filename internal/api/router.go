package api

import "net/http"

// NewRouter creates an HTTP router with all tracking endpoints
func NewRouter(publisher EventPublisher) http.Handler {
	mux := http.NewServeMux()

	// Branded primary endpoint
	mux.Handle("/track", TrackHandler(publisher))

	// Matomo SDK compatibility alias
	mux.Handle("/matomo.php", TrackHandler(publisher))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	return mux
}
