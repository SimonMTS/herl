package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

// TODO: rename all websocket/ws specific language

var (
	notify    bool
	serve     bool
	origin    string
	proxyBind string
	wsBind    string
	quiet     bool
)

func main() {
	flag.BoolVar(&notify, "notify", false,
		"Notify the proxy server that it should reload.")
	flag.BoolVar(&notify, "n", false, "")

	flag.BoolVar(&serve, "serve", false,
		"Start the proxy server.")

	flag.StringVar(&origin, "origin",
		"http://127.0.0.1:8080",
		"Where requests should be sent.\n"+
			"This should be the address of your application.\n")

	flag.StringVar(&proxyBind, "addr",
		"127.0.0.1:3030",
		"The address the proxy server binds to.\n"+
			"This is what you open in the browser.\n")

	flag.StringVar(&wsBind, "ws-addr",
		"127.0.0.1:3031",
		"The address the websocket server binds to.\n")

	flag.BoolVar(&quiet, "quiet", false,
		"Do not output anything to stdout.")

	flag.Parse()

	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if serve && notify {
		return errors.New(
			"-serve and -notify (-n) are mutually exclusive" +
				"see -help for details")
	}

	switch {
	default:
		return errors.New(
			"one of -serve or -notify (-n) must be specified, " +
				"see -help for details")
	case notify:
		return doNotify()
	case serve:
		return doServe()
	}
}
