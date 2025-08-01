package main

import (
	"context"
	"io"
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

	resp, err := libHttp.ExecuteHTTPRequest(http.DefaultClient, req)
	if err != nil {
		panic(errors.Wrap(err, "Failed to libHttp.ExecuteHTTPRequest"))
	}
	defer func(Body io.ReadCloser) {
		if closeErr := Body.Close(); closeErr != nil {
			panic(errors.Wrap(closeErr, "Failed to Close"))
		}
	}(resp.Body)

	log.Println("Health check passed")
}
