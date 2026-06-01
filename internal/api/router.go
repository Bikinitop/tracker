package api

import "net/http"

// NewRouter creates an HTTP router with all tracking endpoints
func NewRouter(publisher EventPublisher) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/matomo.php", TrackHandler(publisher))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	return mux
}
