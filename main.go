package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

func main() {
	var notifyFlag, serveFlag, quietFlag bool
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

	flag.Parse()

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

	urls := URLs{}
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
		return serve(quietFlag, urls)
	default:
		return errors.New(
			"one of -serve or -notify (-n) must be specified, " +
				"see -help for details")
	}
}

func notify(url *url.URL) error {
	resp, err := http.Post(url.String(), "", nil)
	if err != nil {
		return errors.Join(
			errors.New("failed to reach the notification server, "+
				"if it is running on a non-standard\n"+
				"address you can set it here with the -notif-addr flag"),
			err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf(
			"something went wrong with the notification server: %s",
			resp.Status)
	}
	return nil
}

func serve(quiet bool, urls URLs) (err error) {
	wait := make(chan struct{}, 2)
	// If either of these go routines fail we should return

	go func() {
		err = startProxyServer(quiet, urls)
		wait <- struct{}{}
	}()

	go func() {
		err = startNotifServer(urls.Notification)
		wait <- struct{}{}
	}()

	<-wait
	return err
}
