package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/dunglas/frankenphp"
)

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))


	if err := frankenphp.Init(frankenphp.WithContext(ctx), frankenphp.WithLogger(logger)); err != nil {
		panic(err)
	}
	defer frankenphp.Shutdown()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := frankenphp.ServeHTTP(w, r); err != nil {
			panic(err)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if logger.Enabled(ctx, slog.LevelError) {
		logger.LogAttrs(ctx, slog.LevelError, "server error", slog.Any("error", http.ListenAndServe(":"+port, nil)))
	}

	os.Exit(1)
}
