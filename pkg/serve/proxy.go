package serve

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

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

func proxyServer(quiet bool, wsBind string, proxyBind string, origin string) error {
	script := fmt.Appendf(nil, scriptTmpl, wsBind)

	proxyMux := http.NewServeMux()

	proxyMux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		err := prepRequest(req, origin)
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

func prepRequest(req *http.Request, origin string) error {
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
