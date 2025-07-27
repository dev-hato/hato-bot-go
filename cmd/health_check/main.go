package main

import (
	"io"
	"log"
	"net/http"

	"github.com/cockroachdb/errors"
)

func main() {
	// localhost:8080/statusにHTTPリクエストを送信
	resp, err := http.Get("http://localhost:8080/status")
	if err != nil {
		panic(errors.Wrap(err, "Failed to http.Get"))
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
