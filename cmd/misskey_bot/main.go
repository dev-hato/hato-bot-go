package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hato-bot-go/lib/amesh"
	"hato-bot-go/lib/misskey"
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

// handleHTTPResponse HTTPレスポンスの共通処理を行う（JSONデコード用）
func handleHTTPResponseWithJSON(resp *http.Response, target interface{}) error {
	defer func(Body io.ReadCloser) {
		if closeErr := Body.Close(); closeErr != nil {
			panic(errors.Wrap(closeErr, "Failed to Close"))
		}
	}(resp.Body)

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return errors.Wrap(err, "Failed to json.NewDecoder")
	}
	return nil
}

// checkStatusAndDecodeJSON ステータスコードをチェックしJSONをデコードする共通処理
func checkStatusAndDecodeJSON(resp *http.Response, target interface{}) error {
	if resp.StatusCode != 200 {
		if err := resp.Body.Close(); err != nil {
			return errors.Wrap(err, "Failed to Close")
		}
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	if err := handleHTTPResponseWithJSON(resp, target); err != nil {
		return errors.Wrap(err, "Failed to handleHTTPResponseWithJSON")
	}

	return nil
}

// MisskeyBot Misskeyボットクライアント
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

// Location 位置情報の構造体
type Location struct {
	Lat       float64 // 緯度
	Lng       float64 // 経度
	PlaceName string  // 地名
}

// CreateNoteRequest ノート作成のリクエスト構造体
type CreateNoteRequest struct {
	Text         string        // ノートのテキスト
	FileIDs      []string      // 添付ファイルのID一覧
	OriginalNote *misskey.Note // 返信元のノート
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
		ID   string       `json:"id"`
		Type string       `json:"type"`
		Body misskey.Note `json:"body"`
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
		return nil, errors.Wrap(err, "Failed to json.Marshal")
	}

	url := fmt.Sprintf("https://%s/api/%s", bot.Domain, endpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to http.NewRequest")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", bot.UserAgent)

	resp, err := bot.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to Do")
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
		return errors.Wrap(err, "Failed to Dial")
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
		return errors.Wrap(err, "Failed to WriteJSON")
	}

	log.Printf("Connected to Misskey WebSocket: %s", bot.Domain)
	return nil
}

// Listen WebSocketメッセージを監視
func (bot *MisskeyBot) Listen(messageHandler func(note *misskey.Note)) error {
	if messageHandler == nil {
		return errors.New("messageHandler cannot be nil")
	}

	for {
		var msg StreamingMessage
		if err := bot.wsConn.ReadJSON(&msg); err != nil {
			return errors.Wrap(err, "Failed to ReadJSON")
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
func (bot *MisskeyBot) CreateNote(req *CreateNoteRequest) (*misskey.Note, error) {
	if req == nil {
		return nil, errors.New("req cannot be nil")
	}
	if req.OriginalNote == nil {
		return nil, errors.New("originalNote cannot be nil")
	}

	// noteから必要な情報を取得
	visibility := req.OriginalNote.Visibility
	replyID := req.OriginalNote.ID

	// 公開範囲がpublicならばhomeにする
	if visibility == "public" {
		visibility = "home"
	}

	data := map[string]interface{}{
		"text":       req.Text,
		"visibility": visibility,
	}

	if replyID != "" {
		data["replyId"] = replyID
	}

	if len(req.FileIDs) > 0 {
		data["fileIds"] = req.FileIDs
	}

	// 元の投稿がCWされていた場合、それに合わせてCW投稿する
	if req.OriginalNote.CW != nil {
		data["cw"] = "隠すっぽ！"
	}

	resp, err := bot.apiRequest("notes/create", data)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to apiRequest")
	}

	var result struct {
		CreatedNote misskey.Note `json:"createdNote"`
	}

	if err := checkStatusAndDecodeJSON(resp, &result); err != nil {
		return nil, errors.Wrap(err, "Failed to checkStatusAndDecodeJSON")
	}

	return &result.CreatedNote, nil
}

// UploadFile ファイルをアップロード
func (bot *MisskeyBot) UploadFile(filePath string) (*MisskeyFile, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to os.Open")
	}
	defer func(file *os.File) {
		if closeErr := file.Close(); closeErr != nil {
			panic(errors.Wrap(closeErr, "Failed to Close"))
		}
	}(file)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// トークンフィールドを追加
	if writeErr := writer.WriteField("i", bot.Token); writeErr != nil {
		return nil, errors.Wrap(writeErr, "Failed to WriteField")
	}

	// ファイルフィールドを追加
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to CreateFormFile")
	}

	if _, copyErr := io.Copy(part, file); copyErr != nil {
		return nil, errors.Wrap(copyErr, "Failed to io.Copy")
	}

	if closeErr := writer.Close(); closeErr != nil {
		return nil, errors.Wrap(closeErr, "Failed to Close")
	}

	url := fmt.Sprintf("https://%s/api/drive/files/create", bot.Domain)
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to http.NewRequest")
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", bot.UserAgent)

	resp, err := bot.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to Do")
	}

	var uploadedFile MisskeyFile
	if err := checkStatusAndDecodeJSON(resp, &uploadedFile); err != nil {
		return nil, errors.Wrap(err, "Faild to checkStatusAndDecodeJSON")
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
		return errors.Wrap(err, "Failed to apiRequest")
	}
	defer func(Body io.ReadCloser) {
		if closeErr := Body.Close(); closeErr != nil {
			panic(errors.Wrap(closeErr, "Failed to Close"))
		}
	}(resp.Body)

	if resp.StatusCode != 204 {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}

// parseLocation 地名文字列から位置を解析し、Location構造体とエラーを返す
func (bot *MisskeyBot) parseLocation(place, apiKey string) (*Location, error) {
	// 座標が直接提供されているかチェック
	parts := strings.Fields(place)
	if len(parts) == 2 {
		if parsedLat, err1 := parseFloat64(parts[0]); err1 == nil {
			if parsedLng, err2 := parseFloat64(parts[1]); err2 == nil {
				return &Location{
					Lat:       parsedLat,
					Lng:       parsedLng,
					PlaceName: fmt.Sprintf("%.2f,%.2f", parsedLat, parsedLng),
				}, nil
			}
		}
	}

	// 地名をジオコーディング
	result, geocodeErr := amesh.GeocodePlace(place, apiKey)
	if geocodeErr != nil {
		return nil, errors.Wrap(geocodeErr, "Failed to amesh.GeocodePlace")
	}
	return &Location{
		Lat:       result.Lat,
		Lng:       result.Lng,
		PlaceName: result.Name,
	}, nil
}

// createAndSaveImage amesh画像を作成して一時ファイルに保存する
func (bot *MisskeyBot) createAndSaveImage(location *Location) (string, error) {
	if location == nil {
		return "", errors.New("location cannot be nil")
	}
	img, err := amesh.CreateAmeshImage(&amesh.CreateImageRequest{
		Lat:         location.Lat,
		Lng:         location.Lng,
		Zoom:        10,
		AroundTiles: 2,
	})
	if err != nil {
		return "", errors.Wrap(err, "Failed to amesh.CreateAmeshImage")
	}

	filename := fmt.Sprintf("amesh_%s_%d.png", strings.ReplaceAll(location.PlaceName, " ", "_"), time.Now().Unix())
	filePath := "/tmp/" + filename

	file, err := os.Create(filePath)
	if err != nil {
		return "", errors.Wrap(err, "Failed to os.Create")
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			panic(errors.Wrap(closeErr, "Failed to Close"))
		}
	}()

	if err := png.Encode(file, img); err != nil {
		return "", errors.Wrap(err, "Failed to png.Encode")
	}

	return filePath, nil
}

// ProcessAmeshCommand ameshコマンドを処理
func (bot *MisskeyBot) ProcessAmeshCommand(note *misskey.Note, place string) error {
	if note == nil {
		return errors.New("note cannot be nil")
	}

	// 処理中リアクションを追加
	if err := bot.AddReaction(note.ID, "👀"); err != nil {
		return errors.Wrap(err, "Failed to AddReaction")
	}

	// Yahoo APIキーを取得
	apiKey := os.Getenv("YAHOO_API_TOKEN")
	if apiKey == "" {
		return errors.New("YAHOO_API_TOKEN environment variable not set")
	}

	// 位置を解析
	location, err := bot.parseLocation(place, apiKey)
	if err != nil {
		return errors.Wrap(err, "Failed to parseLocation")
	}

	// 画像を作成して保存
	filePath, err := bot.createAndSaveImage(location)
	if err != nil {
		return errors.Wrap(err, "Failed to createAndSaveImage")
	}

	// Misskeyにファイルをアップロード
	uploadedFile, err := bot.UploadFile(filePath)
	if err != nil {
		return errors.Wrap(err, "Failed to UploadFile")
	}

	// 結果をノートとして投稿
	text := fmt.Sprintf("📡 %s (%.4f, %.4f) の雨雲レーダー画像だっぽ", location.PlaceName, location.Lat, location.Lng)
	if _, err := bot.CreateNote(&CreateNoteRequest{
		Text:         text,
		FileIDs:      []string{uploadedFile.ID},
		OriginalNote: note,
	}); err != nil {
		return errors.Wrap(err, "Failed to CreateNote")
	}

	log.Printf("Successfully processed amesh command for %s", location.PlaceName)
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
	messageHandler := func(note *misskey.Note) {
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
			if _, replyErr := bot.CreateNote(&CreateNoteRequest{
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
