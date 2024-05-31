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

var script []byte = []byte(`
<script>
	/* Added by herl */
	document.addEventListener("DOMContentLoaded", () =>
		(new EventSource("http://127.0.0.1:3031/herl-events"))
			.addEventListener("refresh", () => location.reload()))
</script>
`)

// TODO: split up more
// TODO: error handling
func doServe() error {
	wg := sync.WaitGroup{}

	// Start proxy server
	wg.Add(1)
	go func() error {
		proxyMux := http.NewServeMux()
		proxyMux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			url, _ := url.Parse(origin)
			req.URL.Scheme = url.Scheme
			req.URL.Host = url.Host
			req.URL.Path = url.Path
			req.RequestURI = ""
			req.Header.Set("X-Forwarded-For", req.RemoteAddr)

			var resp *http.Response
			var err error
			for i := range 100000 {
				resp, err = http.DefaultClient.Do(req)
				if err == nil {
					_ = i
					// fmt.Println(i)
					break
				}
				time.Sleep(time.Millisecond * 10)
			}
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
			body = bytes.Replace(body, []byte("</body>"), append(script, []byte("</body>")...), 1)
			_, err = w.Write(body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		})
		fmt.Println("herl: proxy server is running on:", proxyBind)
		err := http.ListenAndServe(proxyBind, proxyMux)
		if err != nil {
			return errors.Join(
				errors.New("failed to bind to proxy address, "+
					"this address can be set with the -addr flag"),
				err)
		}
		return nil
	}()

	// Start websocket server
	wg.Add(1)
	go func() error {
		wsMux := http.NewServeMux()
		wsMux.HandleFunc("POST /{$}", func(w http.ResponseWriter, r *http.Request) {
			// fmt.Println("refreshing...")
			select {
			case tmp <- struct{}{}:
			default:
			}
		})
		wsMux.HandleFunc("GET /herl-events", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Type")

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")

			w.WriteHeader(200)

			_, _ = w.Write([]byte("event: connect\nid: 0\ndata: \n\n"))
			w.(http.Flusher).Flush()

			i := 0
			for {
				i++
				select {
				case <-r.Context().Done():
					return
				case <-tmp:
					_, _ = fmt.Fprintf(w, "event: refresh\nid: %d\ndata: \n\n", i)
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
	}()

	wg.Wait()
	return nil
}

var tmp = make(chan struct{})
