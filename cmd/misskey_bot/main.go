package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/massongit/hato-bot-go/lib/amesh"
	"github.com/massongit/hato-bot-go/lib/misskey"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gorilla/websocket"
)

// MisskeyBot Misskey bot client
type MisskeyBot struct {
	Domain    string
	Token     string
	UserAgent string
	client    *http.Client
	wsConn    *websocket.Conn
}

// MisskeyFile アップロードされたファイルの構造体
type MisskeyFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// WebSocketMessage WebSocketメッセージの構造体
type WebSocketMessage struct {
	Type string      `json:"type"`
	Body interface{} `json:"body,omitempty"`
}

// StreamingMessage ストリーミングメッセージの構造体
type StreamingMessage struct {
	Type string `json:"type"`
	Body struct {
		ID   string              `json:"id"`
		Type string              `json:"type"`
		Body misskey.MisskeyNote `json:"body"`
	} `json:"body"`
}

// NewMisskeyBot 新しいMisskeyBotインスタンスを作成
func NewMisskeyBot(domain, token string) *MisskeyBot {
	return &MisskeyBot{
		Domain:    domain,
		Token:     token,
		UserAgent: "hato-bot-go/" + amesh.Version,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// apiRequest MisskeyAPIリクエストを送信
func (bot *MisskeyBot) apiRequest(endpoint string, data interface{}) (*http.Response, error) {
	// データにトークンを追加
	payload := map[string]interface{}{
		"i": bot.Token,
	}

	if data != nil {
		if dataMap, ok := data.(map[string]interface{}); ok {
			for k, v := range dataMap {
				payload[k] = v
			}
		}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal JSON")
	}

	url := fmt.Sprintf("https://%s/api/%s", bot.Domain, endpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", bot.UserAgent)

	resp, err := bot.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request")
	}

	return resp, nil
}

// Connect WebSocket接続を確立
func (bot *MisskeyBot) Connect() error {
	wsURL := fmt.Sprintf("wss://%s/streaming?i=%s", bot.Domain, bot.Token)

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(wsURL, http.Header{
		"User-Agent": []string{bot.UserAgent},
	})
	if err != nil {
		return errors.Wrap(err, "failed to connect to WebSocket")
	}

	bot.wsConn = conn

	// メインチャンネルに接続
	connectMsg := WebSocketMessage{
		Type: "connect",
		Body: map[string]interface{}{
			"channel": "main",
			"id":      "main",
		},
	}

	if err := bot.wsConn.WriteJSON(connectMsg); err != nil {
		return errors.Wrap(err, "failed to send connect message")
	}

	log.Printf("Connected to Misskey WebSocket: %s", bot.Domain)
	return nil
}

// Listen WebSocketメッセージを監視
func (bot *MisskeyBot) Listen(messageHandler func(note *misskey.MisskeyNote)) error {
	if messageHandler == nil {
		return errors.New("messageHandler cannot be nil")
	}

	for {
		var msg StreamingMessage
		if err := bot.wsConn.ReadJSON(&msg); err != nil {
			return errors.Wrap(err, "failed to read WebSocket message")
		}

		// メンションイベントの処理
		if msg.Type != "channel" || msg.Body.Type != "mention" {
			continue
		}

		note := msg.Body.Body
		log.Printf("Received mention from @%s: %s", note.User.Username, note.Text)

		// メッセージハンドラーを呼び出し
		messageHandler(&note)
	}
}

// CreateNote ノートを作成
func (bot *MisskeyBot) CreateNote(text string, fileIDs []string, originalNote *misskey.MisskeyNote) (*misskey.MisskeyNote, error) {
	if originalNote == nil {
		return nil, errors.New("originalNote cannot be nil")
	}

	// noteから必要な情報を取得
	visibility := originalNote.Visibility
	replyID := originalNote.ID

	// 公開範囲がpublicならばhomeにする
	if visibility == "public" {
		visibility = "home"
	}

	data := map[string]interface{}{
		"text":       text,
		"visibility": visibility,
	}

	if replyID != "" {
		data["replyId"] = replyID
	}

	if len(fileIDs) > 0 {
		data["fileIds"] = fileIDs
	}

	// 元の投稿がCWされていた場合、それに合わせてCW投稿する
	if originalNote.CW != nil {
		data["cw"] = "隠すっぽ！"
	}

	resp, err := bot.apiRequest("notes/create", data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create note")
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			panic(err)
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result struct {
		CreatedNote misskey.MisskeyNote `json:"createdNote"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.Wrap(err, "failed to decode response")
	}

	return &result.CreatedNote, nil
}

// UploadFile ファイルをアップロード
func (bot *MisskeyBot) UploadFile(filePath string) (*MisskeyFile, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file")
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			panic(err)
		}
	}(file)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// トークンフィールドを追加
	if err := writer.WriteField("i", bot.Token); err != nil {
		return nil, errors.Wrap(err, "failed to write token field")
	}

	// ファイルフィールドを追加
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create form file")
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, errors.Wrap(err, "failed to copy file")
	}

	if err := writer.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to close writer")
	}

	url := fmt.Sprintf("https://%s/api/drive/files/create", bot.Domain)
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", bot.UserAgent)

	resp, err := bot.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request")
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			panic(err)
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var uploadedFile MisskeyFile
	if err := json.NewDecoder(resp.Body).Decode(&uploadedFile); err != nil {
		return nil, errors.Wrap(err, "failed to decode response")
	}

	return &uploadedFile, nil
}

// AddReaction リアクションを追加
func (bot *MisskeyBot) AddReaction(noteID, reaction string) error {
	data := map[string]interface{}{
		"noteId":   noteID,
		"reaction": reaction,
	}

	resp, err := bot.apiRequest("notes/reactions/create", data)
	if err != nil {
		return errors.Wrap(err, "failed to add reaction")
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			panic(err)
		}
	}(resp.Body)

	if resp.StatusCode != 204 {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}

// ProcessAmeshCommand ameshコマンドを処理
func (bot *MisskeyBot) ProcessAmeshCommand(note *misskey.MisskeyNote, place string) error {
	if note == nil {
		return errors.New("note cannot be nil")
	}

	// 処理中のリアクションを追加
	if err := bot.AddReaction(note.ID, "👀"); err != nil {
		log.Printf("Failed to add reaction: %v", err)
	}

	// Yahoo APIキーを取得
	apiKey := os.Getenv("YAHOO_API_TOKEN")
	if apiKey == "" {
		return errors.New("YAHOO_API_TOKEN environment variable not set")
	}

	// 地名から座標を取得
	var lat, lng float64
	var placeName string
	var err error

	// 座標が直接提供された場合の解析
	parts := strings.Fields(place)
	if len(parts) == 2 {
		// 座標として解析を試行
		if parsedLat, err1 := parseFloat64(parts[0]); err1 == nil {
			if parsedLng, err2 := parseFloat64(parts[1]); err2 == nil {
				lat, lng = parsedLat, parsedLng
				placeName = fmt.Sprintf("%.2f,%.2f", lat, lng)
			}
		}
	}

	if placeName == "" {
		// 地名をジオコーディング
		result, err := amesh.GeocodePlace(place, apiKey)
		if err != nil {
			return errors.Wrap(err, "failed to geocode place")
		}
		lat, lng, placeName = result.Lat, result.Lng, result.Name
	}

	// amesh画像を生成
	img, err := amesh.CreateAmeshImage(lat, lng, 10, 2)
	if err != nil {
		return errors.Wrap(err, "failed to create amesh image")
	}

	// 一時ファイルに保存
	filename := fmt.Sprintf("amesh_%s_%d.png", strings.ReplaceAll(placeName, " ", "_"), time.Now().Unix())
	filePath := "/tmp/" + filename

	file, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create temporary file")
	}
	defer func() {
		// ファイルを先にクローズしてから削除
		if err := file.Close(); err != nil {
			panic(err)
		}
		if err := os.Remove(filePath); err != nil {
			panic(err)
		}
	}()

	if err := png.Encode(file, img); err != nil {
		return errors.Wrap(err, "failed to encode PNG")
	}

	// Misskeyにファイルをアップロード
	uploadedFile, err := bot.UploadFile(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to upload file to Misskey")
	}

	// 結果をノートとして投稿
	text := fmt.Sprintf("📡 %s (%.4f, %.4f) の雨雲レーダー画像だっぽ", placeName, lat, lng)

	if _, err := bot.CreateNote(text, []string{uploadedFile.ID}, note); err != nil {
		return errors.Wrap(err, "failed to create reply note")
	}

	log.Printf("Successfully processed amesh command for %s", placeName)
	return nil
}

// parseFloat64 文字列をfloat64に変換
func parseFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// statusHandler /statusエンドポイントのハンドラー
func statusHandler(w http.ResponseWriter, _ *http.Request) {
	response := map[string]interface{}{
		"message": "hato-bot-go is running",
		"version": amesh.Version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Failed to Encode: %v", err)
	}
}

// startHTTPServer HTTPサーバーを開始
func startHTTPServer() {
	http.HandleFunc("/status", statusHandler)

	port := "8080"
	log.Printf("Starting HTTP server on port %s", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Printf("HTTP server error: %v", err)
	}
}

// main Misskeyボットとして実行
func main() {
	// 環境変数から設定を取得
	domain := os.Getenv("MISSKEY_DOMAIN")
	token := os.Getenv("MISSKEY_API_TOKEN")

	if domain == "" || token == "" {
		log.Fatal("MISSKEY_DOMAIN and MISSKEY_API_TOKEN environment variables must be set")
	}

	// Yahoo APIキーも必要
	if os.Getenv("YAHOO_API_TOKEN") == "" {
		log.Fatal("YAHOO_API_TOKEN environment variable must be set")
	}

	// HTTPサーバーを別ゴルーチンで開始
	go startHTTPServer()

	// ボットを初期化
	bot := NewMisskeyBot(domain, token)

	// WebSocket接続を確立
	if err := bot.Connect(); err != nil {
		log.Fatalf("Failed to connect to Misskey: %v", err)
	}

	log.Printf("hato-bot-go started on %s", domain)

	// メッセージハンドラー
	messageHandler := func(note *misskey.MisskeyNote) {
		// ameshコマンドを解析
		parseResult := misskey.ParseAmeshCommand(note.Text)

		if !parseResult.IsAmesh {
			return
		}

		log.Printf("Processing amesh command for place: %s", parseResult.Place)

		// ameshコマンドを処理
		if err := bot.ProcessAmeshCommand(note, parseResult.Place); err != nil {
			log.Printf("Error processing amesh command: %v", err)

			// エラーメッセージを投稿
			errorMsg := fmt.Sprintf("申し訳ないっぽ。ameshコマンドの処理中にエラーが発生したっぽ: %v", err)
			if _, replyErr := bot.CreateNote(errorMsg, nil, note); replyErr != nil {
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
