package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/assaidy/url_shortener/config"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.Info("new request", "path", "/")
		w.Write([]byte("Hello, World!\n"))
	})

	slog.Info("server started", "addr", config.ServerAddr)
	if err := http.ListenAndServe(config.ServerAddr, nil); err != nil {
		slog.Error("err starting server", "err", err)
		os.Exit(1)
	}
}
