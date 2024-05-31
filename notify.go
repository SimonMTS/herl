package main

import (
	"errors"
	"net/http"
	"strings"
)

// TODO: error handling
func doNotify() error {
	resp, err := http.Post("http://127.0.0.1:3031/", "", strings.NewReader(""))
	if err != nil {
		return errors.Join(
			errors.New("TODO"),
			err)
	}
	_ = resp
	return nil
}
