package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// TODO: append if there is no body
// TODO: variable naming

type Urls struct {
	Notification,
	Proxy,
	Origin *url.URL
}

const (
	retryTimeout = time.Millisecond * 50
	retryMaxTime = time.Second * 5
	retries      = int(retryMaxTime / retryTimeout)
)

const scriptTmpl = `
<script>
	/* Added by herl */
	document.addEventListener("DOMContentLoaded", () =>
		(new EventSource("%s/herl-events"))
			.addEventListener("refresh", () => location.reload()))
</script>
</body>
`

func Proxy(quiet bool, addrs Urls) error {
	script := fmt.Appendf(nil, scriptTmpl, addrs.Notification)

	proxyMux := http.NewServeMux()
	proxyMux.HandleFunc("/", proxyHandler(script, addrs.Origin))

	if !quiet {
		fmt.Println("herl: proxy listening on:", addrs.Proxy)
	}
	slog.Debug("starting proxy server",
		"addr", addrs.Proxy.Host)

	err := http.ListenAndServe(addrs.Proxy.Host, proxyMux)
	if err != nil {
		return errors.Join(
			errors.New("failed to bind to proxy address, "+
				"this address can be set with the -addr flag"),
			err)
	}

	return nil
}

func proxyHandler(
	script []byte,
	origin *url.URL,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		slog.Debug("received request on proxy",
			"host", req.Host,
			"url", req.URL.String())

		req.URL.Scheme = origin.Scheme
		req.URL.Host = origin.Host
		req.URL.Path = origin.Path
		req.RequestURI = ""
		req.Header.Set("X-Forwarded-For", req.RemoteAddr)

		resp, err := callOrigin(req)
		if err != nil {
			slog.Debug("call to origin server failed",
				"error", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		slog.Debug("call to origin server succeeded",
			"status", resp.Status)

		// TODO: copy over headers

		w.WriteHeader(resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			slog.Debug("failed to read origin response body",
				"error", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		newBody := bytes.Replace(body, []byte("</body>"), script, 1)
		slog.Debug("added script to origin body",
			"old", string(body),
			"new", string(newBody))

		_, err = w.Write(newBody)
		if err != nil {
			slog.Debug("failed write response",
				"error", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		slog.Debug("successfully handled proxy request")
	}
}

func callOrigin(req *http.Request) (resp *http.Response, err error) {
	for i := range retries {

		slog.Debug("call origin server",
			"url", req.URL,
			"attempt", i,
			"max attempts", retries,
			"timeout between attempts", retryTimeout.String())

		resp, err = http.DefaultClient.Do(req)
		if err == nil {
			return resp, nil
		}

		slog.Debug("attempt to call origin server failed",
			"error", err.Error())

		time.Sleep(retryTimeout)
	}
	return nil, err
}
