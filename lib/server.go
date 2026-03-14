package lib

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// statusHandler /statusエンドポイントのハンドラー
func statusHandler(w http.ResponseWriter, _ *http.Request) {
	response := map[string]string{
		"message": "hato-bot-go is running",
		"version": Version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to Encode: %v", err)
	}
}

// StartStatusHTTPServer HTTPサーバーを開始
func StartStatusHTTPServer() {
	http.HandleFunc("/status", statusHandler)

	port := "8080"
	log.Printf("Starting HTTP server on port %s", port)

	server := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Printf("HTTP server error: %v", err)
	}
}
