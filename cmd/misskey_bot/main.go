package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"hato-bot-go/lib/amesh"
	"hato-bot-go/lib/misskey"
)

// main Misskeyボットとして実行
func main() {
	// 環境変数から設定を取得
	domain := os.Getenv("MISSKEY_DOMAIN")
	token := os.Getenv("MISSKEY_API_TOKEN")

	if domain == "" || token == "" {
		log.Fatal("MISSKEY_DOMAIN and MISSKEY_API_TOKEN environment variables must be set")
	}

	yahooAPIToken := os.Getenv("YAHOO_API_TOKEN")

	// Yahoo APIキーも必要
	if yahooAPIToken == "" {
		log.Fatal("YAHOO_API_TOKEN environment variable must be set")
	}

	// HTTPサーバーを別ゴルーチンで開始
	go startHTTPServer()

	// ボットを初期化
	bot := misskey.NewBot(domain, token)

	// WebSocket接続を確立
	if err := bot.Connect(); err != nil {
		log.Fatalf("Failed to connect to Misskey: %v", err)
	}

	log.Printf("hato-bot-go started on %s", domain)

	// メッセージハンドラー
	messageHandler := func(note *misskey.Note) {
		// ameshコマンドを解析
		parseResult := misskey.ParseAmeshCommand(note.Text)

		if !parseResult.IsAmesh {
			return
		}

		log.Printf("Processing amesh command for place: %s", parseResult.Place)
		ctx := context.Background()

		// ameshコマンドを処理
		if err := bot.ProcessAmeshCommand(ctx, &misskey.ProcessAmeshCommandParams{
			Note:          note,
			Place:         parseResult.Place,
			YahooAPIToken: yahooAPIToken,
		}); err != nil {
			log.Printf("Error processing amesh command: %v", err)

			// エラーメッセージを投稿
			if replyErr := bot.CreateNote(ctx, &misskey.CreateNoteParams{
				Text:         "申し訳ないっぽ。ameshコマンドの処理中にエラーが発生したっぽ",
				FileIDs:      nil,
				OriginalNote: note,
			}); replyErr != nil {
				log.Printf("Failed to send error message: %v", replyErr)
			}
		}
	}

	// WebSocketメッセージを監視
	for {
		if err := bot.Listen(messageHandler); err != nil {
			log.Printf("WebSocket connection lost: %v", err)
			log.Println("Attempting to reconnect...")

			// 再接続を試行
			time.Sleep(5 * time.Second)
			if err = bot.Connect(); err != nil {
				log.Printf("Failed to reconnect: %v", err)
				time.Sleep(10 * time.Second)
			}
		}
	}
}

// statusHandler /statusエンドポイントのハンドラー
func statusHandler(w http.ResponseWriter, _ *http.Request) {
	response := map[string]any{
		"message": "hato-bot-go is running",
		"version": amesh.Version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to Encode: %v", err)
	}
}

// startHTTPServer HTTPサーバーを開始
func startHTTPServer() {
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
