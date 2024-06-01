package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"
)

var (
	events    = make(chan struct{})
	listeners = atomic.Int32{}
)

func startNotifServer(notifUrl *url.URL) error {
	notifMux := http.NewServeMux()
	notifMux.HandleFunc("POST /{$}", notifHandler)
	notifMux.HandleFunc("GET /herl-events", eventsHandler)

	err := http.ListenAndServe(notifUrl.Host, notifMux)
	if err != nil {
		return errors.Join(
			errors.New("failed to bind to notification server address, "+
				"this address can be set with the -notif-addr flag"),
			err)
	}
	return nil
}

func notifHandler(w http.ResponseWriter, r *http.Request) {
	listenerCount := listeners.Load()
	for range listenerCount {
		select {
		case events <- struct{}{}:
		default:
			// If the channel is blocked, just continue
		}
	}
	w.WriteHeader(http.StatusOK)
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
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

	listeners.Add(1)
	defer func() { listeners.Add(-1) }()

	for i := 0; true; i++ {
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
}
