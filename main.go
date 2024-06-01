package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sync"

	"s14.nl/herl/server"
)

func main() {
	var notifyFlag, serveFlag, quietFlag, debugFlag bool
	var originUrl, proxyUrl, notifUrl string

	flag.BoolVar(&notifyFlag, "notify", false,
		"Notify the proxy server that it should reload.")
	flag.BoolVar(&notifyFlag, "n", false, "")

	flag.BoolVar(&serveFlag, "serve", false,
		"Start the proxy server.")

	flag.StringVar(&originUrl, "origin",
		"http://127.0.0.1:8080",
		"The url of the origin server, where requests are sent.\n"+
			"This should be the address of your application.\n")

	flag.StringVar(&proxyUrl, "addr",
		"http://127.0.0.1:3030",
		"The url the proxy server will listen to.\n"+
			"This is what you open in the browser.\n")

	flag.StringVar(&notifUrl, "notif-addr",
		"http://127.0.0.1:3031",
		"The url the notification server will listen to.\n")

	flag.BoolVar(&quietFlag, "quiet", false,
		"Do not output anything to stdout.")

	flag.BoolVar(&debugFlag, "debug", false,
		"Print debug logging (pipe to jq for readable output).")

	flag.Parse()

	if debugFlag {
		quietFlag = true
		slog.SetDefault(slog.New(slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{
				Level: slog.LevelDebug,
			})))
	}

	slog.Debug("parsed flags",
		"notifyFlag", notifyFlag,
		"serveFlag", serveFlag,
		"quietFlag", quietFlag,
		"debugFlag", debugFlag,
		"originUrl", originUrl,
		"proxyUrl", proxyUrl,
		"notifUrl", notifUrl)

	err := run(
		notifyFlag, serveFlag, quietFlag,
		originUrl, proxyUrl, notifUrl)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(
	notifyFlag, serveFlag, quietFlag bool,
	originUrl, proxyUrl, notifUrl string,
) error {
	if serveFlag && notifyFlag {
		return errors.New(
			"-serve and -notify (-n) are mutually exclusive" +
				"see -help for details")
	}

	urls := server.Urls{}
	err := error(nil)

	urls.Origin, err = url.Parse(originUrl)
	if err != nil {
		return fmt.Errorf(
			"origin server url is not valid:\n%w", err)
	}

	urls.Proxy, err = url.Parse(proxyUrl)
	if err != nil {
		return fmt.Errorf(
			"proxy server url is not valid:\n%w", err)
	}

	urls.Notification, err = url.Parse(notifUrl)
	if err != nil {
		return fmt.Errorf(
			"notification server url is not valid:\n%w", err)
	}

	switch {
	case notifyFlag:
		return notify(urls.Notification)
	case serveFlag:
		return Serve(quietFlag, urls)
	default:
		return errors.New(
			"one of -serve or -notify (-n) must be specified, " +
				"see -help for details")
	}
}

func notify(url *url.URL) error {
	slog.Debug("sending notification post", "url", url)
	resp, err := http.Post(url.String(), "", nil)
	if err != nil {
		return errors.Join(
			errors.New("failed to reach the notification server, "+
				"if it is running on a non-standard\n"+
				"address you can set it here with the -notif-addr flag"),
			err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		slog.Debug("notification post failed",
			"status", resp.Status,
			"body", string(body))
		return fmt.Errorf(
			"something went wrong with the notification server: %s",
			resp.Status)
	}
	slog.Debug("notification post was successful",
		"status", resp.Status)
	return nil
}

func Serve(quiet bool, addrs server.Urls) (err error) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	// If either of these go routines fails we should return

	go func() {
		err = server.Proxy(quiet, addrs)
		wg.Done()
	}()

	go func() {
		err = server.Notification(addrs.Notification)
		wg.Done()
	}()

	wg.Wait()
	return err
}
