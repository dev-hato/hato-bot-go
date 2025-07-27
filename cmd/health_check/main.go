package main

import (
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	// localhost:8080/statusにHTTPリクエストを送信
	resp, err := http.Get("http://localhost:8080/status")
	if err != nil {
		log.Printf("Health check failed: %v", err)
		os.Exit(1)
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			panic(err)
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		log.Printf("Health check failed: HTTP %d", resp.StatusCode)
		os.Exit(1)
	}

	log.Println("Health check passed")
	os.Exit(0)
}
