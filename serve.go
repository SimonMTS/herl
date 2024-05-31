package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// TODO: split up more
// TODO: origin without protocol causes crashes, and probably some more stuff
// TODO: try not to force 'http' in script

const (
	retryTimeout = time.Millisecond * 10
	retryMaxTime = time.Second * 10
	retries      = int(retryMaxTime / retryTimeout)
)

const scriptTmpl = `
<script>
	/* Added by herl */
	document.addEventListener("DOMContentLoaded", () =>
		(new EventSource("http://%s/herl-events"))
			.addEventListener("refresh", () => location.reload()))
</script>
</body>
`

var (
	events    = make(chan struct{})
	listeners = 0
)

func doServe() (err error) {
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		err = proxyServer()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		err = notificationServer()
		wg.Done()
	}()

	wg.Wait()
	return err
}

func proxyServer() error {
	script := fmt.Appendf(nil, scriptTmpl, wsBind)

	proxyMux := http.NewServeMux()

	proxyMux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		err := prepRequest(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp, err := callOrigin(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// TODO: copy over headers

		w.WriteHeader(resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		body = bytes.Replace(body, []byte("</body>"), script, 1)
		_, err = w.Write(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	if !quiet {
		fmt.Println("herl: proxy server is running on:", proxyBind)
	}
	err := http.ListenAndServe(proxyBind, proxyMux)
	if err != nil {
		return errors.Join(
			errors.New("failed to bind to proxy address, "+
				"this address can be set with the -addr flag"),
			err)
	}
	return nil
}

func prepRequest(req *http.Request) error {
	url, err := url.Parse(origin)
	if err != nil {
		return err
	}
	req.URL.Scheme = url.Scheme
	req.URL.Host = url.Host
	req.URL.Path = url.Path
	req.RequestURI = ""
	req.Header.Set("X-Forwarded-For", req.RemoteAddr)
	return nil
}

func callOrigin(req *http.Request) (resp *http.Response, err error) {
	for range retries {
		resp, err = http.DefaultClient.Do(req)
		if err == nil {
			return resp, nil
		}
		time.Sleep(retryTimeout)
	}
	return nil, err
}

func notificationServer() error {
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
