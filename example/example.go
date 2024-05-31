package main

import (
	_ "embed"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"text/template"
)

//go:embed index.html
var html string

func main() {
	fmt.Println("starting...")

	tmpl, err := template.New("index").Parse(html)
	if err != nil {
		slog.Error(err.Error())
		return
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.Info("handling request", "url", r.URL)
		tmpl.Execute(w, rand.Int())
	})

	bind := "127.0.0.1:8080"
	slog.Info("started server", "addr", bind)
	err = http.ListenAndServe(bind, nil)
	slog.Error(err.Error())
}
