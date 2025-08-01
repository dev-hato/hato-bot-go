package main

import (
	"context"
	libHttp "hato-bot-go/lib/http"
	"io"
	"log"
	"net/http"

	"github.com/cockroachdb/errors"
)

func main() {
	// localhost:8080/statusにHTTPリクエストを送信
	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/status", nil)
	if err != nil {
		panic(errors.Wrap(err, "Failed to http.NewRequestWithContext"))
	}
	resp, err := libHttp.ExecuteHTTPRequest(http.DefaultClient, req)
	if err != nil {
		panic(errors.Wrap(err, "Failed to executeHTTPRequest"))
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			panic(errors.Wrap(err, "Failed to Close"))
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		panic(errors.Errorf("Health check failed: HTTP %d", resp.StatusCode))
	}

	log.Println("Health check passed")
}
