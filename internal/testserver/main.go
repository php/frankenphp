package main

import (
	"net/http"
	"os"

	"github.com/dunglas/frankenphp"
)

func main() {
	root := os.Getenv("ROOT")
	if root == "" {
		root = "public"
	}

	server, err := frankenphp.NewServer(root)

	if err != nil {
		panic(err)
	}

	if err := frankenphp.Init(frankenphp.WithServer(server)); err != nil {
		panic(err)
	}
	defer frankenphp.Shutdown()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := server.ServeHTTP(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		panic(err)
	}

	os.Exit(1)
}
