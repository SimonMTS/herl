package serve

import (
	"errors"
	"fmt"
	"net/http"
)

func notificationServer(wsBind string) error {
	wsMux := http.NewServeMux()

	wsMux.HandleFunc("POST /{$}", func(w http.ResponseWriter, _ *http.Request) {
		for range listeners {
			select {
			case events <- struct{}{}:
			default:
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	wsMux.HandleFunc("GET /herl-events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Type")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		w.WriteHeader(http.StatusOK)

		_, err := w.Write([]byte("event: connect\nid: 0\ndata: \n\n"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.(http.Flusher).Flush()

		listeners++
		defer func() { listeners-- }()
		i := 0
		for {
			i++
			select {
			case <-r.Context().Done():
				return
			case <-events:
				_, err := fmt.Fprintf(w, "event: refresh\nid: %d\ndata: \n\n", i)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.(http.Flusher).Flush()
			}
		}
	})

	err := http.ListenAndServe(wsBind, wsMux)
	if err != nil {
		return errors.Join(
			errors.New("failed to bind to websocket address, "+
				"this address can be set with the -ws-addr flag"),
			err)
	}
	return nil
}
