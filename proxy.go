package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type URLs struct {
	Notification,
	Proxy,
	Origin *url.URL
}

const (
	retryTimeout = time.Millisecond * 50
	retryMaxTime = time.Second * 5
	retries      = int(retryMaxTime / retryTimeout)

	scriptTmpl = `
	<script>
		/* Added by herl */
		document.addEventListener("DOMContentLoaded", () =>
			(new EventSource("%s/herl-events"))
				.addEventListener("refresh", () => location.reload()))
	</script>
	`
)

var endBodyTag = []byte("</body>")

func startProxyServer(quiet bool, urls URLs) error {
	script := fmt.Appendf(nil, scriptTmpl, urls.Notification)

	proxyMux := http.NewServeMux()
	proxyMux.HandleFunc("/", proxyHandler(script, urls.Origin))

	if !quiet {
		fmt.Println("herl: proxy listening on:", urls.Proxy.Host)
	}

	err := http.ListenAndServe(urls.Proxy.Host, proxyMux)
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
		req.URL.Scheme = origin.Scheme
		req.URL.Host = origin.Host
		req.Host = origin.Host
		req.RequestURI = ""
		req.Header.Set("X-Forwarded-For", req.RemoteAddr)
		// Replacing the body is complicated by compression, so disable it
		req.Header.Set("Accept-Encoding", "")

		resp, err := callOrigin(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for key, vals := range resp.Header {
			if key == "Content-Length" {
				// We are changing the body, so this would be incorrect
				continue
			}
			for _, val := range vals {
				w.Header().Add(key, val)
			}
		}

		body, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ct := resp.Header.Get("Content-Type")
		if strings.Contains(ct, "text/html") {
			if bytes.Contains(body, endBodyTag) {
				body = bytes.Replace(
					body, endBodyTag,
					append(script, endBodyTag...),
					1)
			} else {
				body = append(body, script...)
			}
		}

		w.WriteHeader(resp.StatusCode)
		_, err = w.Write(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
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
