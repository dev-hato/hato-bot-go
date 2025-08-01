package main

import (
	"context"
	"log"
	"net/http"

	"github.com/cockroachdb/errors"

	libHttp "hato-bot-go/lib/http"
)

func main() {
	// localhost:8080/statusにHTTPリクエストを送信
	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/status", nil)
	if err != nil {
		panic(errors.Wrap(err, "Failed to http.NewRequestWithContext"))
	}
	if _, err := libHttp.ExecuteHTTPRequest(http.DefaultClient, req); err != nil {
		panic(errors.Wrap(err, "Failed to libHttp.ExecuteHTTPRequest"))
	}

	log.Println("Health check passed")
}
