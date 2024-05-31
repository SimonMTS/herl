package main

import (
	"errors"
	"fmt"
	"net/http"
)

func doNotify() error {
	resp, err := http.Post("http://"+wsBind, "", nil)
	if err != nil {
		return errors.Join(
			errors.New("failed to reach the notification server, "+
				"if it is running on a non-standard\n"+
				"address you can set it here with the -ws-addr flag"),
			err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf(
			"something went wrong with the notification server: %s",
			resp.Status)
	}
	return nil
}
