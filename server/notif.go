package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync/atomic"
)

var (
	events    = make(chan struct{})
	listeners = atomic.Int32{}
)

func Notification(notifUrl *url.URL) error {
	notifMux := http.NewServeMux()
	notifMux.HandleFunc("POST /{$}", notifHandler)
	notifMux.HandleFunc("GET /herl-events", eventsHandler)

	slog.Debug("starting notification server",
		"addr", notifUrl.Host)
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
	slog.Debug("received notification post",
		"listeners", listenerCount,
		"host", r.Host,
		"url", r.URL.String())
	for range listenerCount {
		select {
		case events <- struct{}{}:
			slog.Debug("sent on events channel")
		default:
			slog.Debug("failed to send on events channel")
		}
	}
	w.WriteHeader(http.StatusOK)
	slog.Debug("notification post finished successfully")
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Type")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	w.WriteHeader(http.StatusOK)

	slog.Debug("received events get",
		"host", r.Host,
		"url", r.URL.String())

	_, err := w.Write([]byte("event: connect\nid: 0\ndata: \n\n"))
	if err != nil {
		slog.Debug("failed to send initial connect event",
			"error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.(http.Flusher).Flush()
	slog.Debug("sent initial connect event")

	n := listeners.Add(1)
	slog.Debug("add listener", "listeners", n)

	defer func() {
		n := listeners.Add(-1)
		slog.Debug("remove listener", "listeners", n)
	}()

	for i := 0; true; i++ {
		select {
		case <-r.Context().Done():
			slog.Debug("event stream closed")
			return
		case <-events:
			_, err := fmt.Fprintf(w, "event: refresh\nid: %d\ndata: \n\n", i)
			if err != nil {
				slog.Debug("failed to send refresh event",
					"error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.(http.Flusher).Flush()
			slog.Debug("sent refresh event")
		}
	}
}
