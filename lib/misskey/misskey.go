package misskey

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hato-bot-go/lib/amesh"
	libHttp "hato-bot-go/lib/http"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gorilla/websocket"
)

// Note Misskeyのノート構造体
type Note struct {
	ID         string   `json:"id"`
	Text       string   `json:"text,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
	FileIDs    []string `json:"fileIds,omitempty"`
	ReplyID    string   `json:"replyId,omitempty"`
	CW         *string  `json:"cw,omitempty"`
	User       struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Host     string `json:"host,omitempty"`
	} `json:"user"`
}

// ParseResult ameshコマンドの解析結果を表す構造体
type ParseResult struct {
	Place   string
	IsAmesh bool
}

// File アップロードされたファイルの構造体
type File struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// CreateNoteRequest ノート作成のリクエスト構造体
type CreateNoteRequest struct {
	Text         string   // ノートのテキスト
	FileIDs      []string // 添付ファイルのID一覧
	OriginalNote *Note    // 返信元のノート
}

// BotRequest ボット作成のリクエスト構造体
type BotRequest struct {
	Domain string         // Misskeyのドメイン
	Token  string         // APIトークン
	Client libHttp.Client // HTTPクライアント
}

// Bot Misskeyボットクライアント
type Bot struct {
	Domain    string
	Token     string
	UserAgent string
	client    libHttp.Client
	wsConn    *websocket.Conn
}

// NewBot 新しいBotインスタンスを作成
func NewBot(domain, token string) *Bot {
	return NewBotWithClient(&BotRequest{
		Domain: domain,
		Token:  token,
		Client: &http.Client{Timeout: 30 * time.Second},
	})
}

// NewBotWithClient HTTPクライアント注入可能なBotインスタンスを作成
func NewBotWithClient(req *BotRequest) *Bot {
	if req == nil {
		return nil
	}
	if req.Client == nil {
		return nil
	}
	return &Bot{
		Domain:    req.Domain,
		Token:     req.Token,
		UserAgent: "hato-bot-go/" + amesh.Version,
		client:    req.Client,
	}
}

// Connect WebSocket接続を確立
func (bot *Bot) Connect() error {
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
	connectMsg := struct {
		Type string      `json:"type"`
		Body interface{} `json:"body,omitempty"`
	}{
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
func (bot *Bot) Listen(messageHandler func(note *Note)) error {
	if messageHandler == nil {
		return errors.New("messageHandler cannot be nil")
	}

	for {
		var msg struct {
			Type string `json:"type"`
			Body struct {
				ID   string `json:"id"`
				Type string `json:"type"`
				Body Note   `json:"body"`
			} `json:"body"`
		}
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
func (bot *Bot) CreateNote(req *CreateNoteRequest) error {
	if req == nil {
		return errors.New("req cannot be nil")
	}
	if req.OriginalNote == nil {
		return errors.New("originalNote cannot be nil")
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

	if 0 < len(req.FileIDs) {
		data["fileIds"] = req.FileIDs
	}

	// 元の投稿がCWされていた場合、それに合わせてCW投稿する
	if req.OriginalNote.CW != nil {
		data["cw"] = "隠すっぽ！"
	}

	resp, err := bot.apiRequest("notes/create", data)
	if err != nil {
		return errors.Wrap(err, "Failed to apiRequest")
	}

	var result struct {
		CreatedNote Note `json:"createdNote"`
	}

	if err := checkStatusAndDecodeJSON(resp, &result); err != nil {
		return errors.Wrap(err, "Failed to checkStatusAndDecodeJSON")
	}

	return nil
}

// UploadFile ファイルをアップロード
func (bot *Bot) UploadFile(filePath string) (*File, error) {
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
		return nil, errors.Wrap(err, "Failed to libHttp.NewRequest")
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", bot.UserAgent)

	resp, err := bot.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to Do")
	}

	var uploadedFile File
	if err := checkStatusAndDecodeJSON(resp, &uploadedFile); err != nil {
		return nil, errors.Wrap(err, "Faild to checkStatusAndDecodeJSON")
	}

	return &uploadedFile, nil
}

// AddReaction リアクションを追加
func (bot *Bot) AddReaction(noteID, reaction string) error {
	data := map[string]interface{}{
		"noteId":   noteID,
		"reaction": reaction,
	}

	resp, err := bot.apiRequest("notes/reactions/create", data)
	if err != nil {
		return errors.Wrap(err, "Failed to apiRequest")
	}
	defer func(body io.ReadCloser) {
		if closeErr := body.Close(); closeErr != nil {
			panic(errors.Wrap(closeErr, "Failed to Close"))
		}
	}(resp.Body)

	if resp.StatusCode != 204 {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}

// ProcessAmeshCommand ameshコマンドを処理
func (bot *Bot) ProcessAmeshCommand(note *Note, place string) error {
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
	location, err := amesh.ParseLocation(place, apiKey)
	if err != nil {
		return errors.Wrap(err, "Failed to amesh.ParseLocation")
	}

	fmt.Printf("Generating amesh image for %s (%.4f, %.4f)\n", location.PlaceName, location.Lat, location.Lng)

	// 画像を作成して保存
	filePath, err := amesh.CreateAndSaveImage(location, "/tmp")
	if err != nil {
		return errors.Wrap(err, "Failed to amesh.CreateAndSaveImage")
	}

	// Misskeyにファイルをアップロード
	uploadedFile, err := bot.UploadFile(filePath)
	if err != nil {
		return errors.Wrap(err, "Failed to UploadFile")
	}

	// 結果をノートとして投稿
	text := fmt.Sprintf(
		"📡 %s (%.4f, %.4f) の雨雲レーダー画像だっぽ",
		location.PlaceName,
		location.Lat,
		location.Lng,
	)
	if err := bot.CreateNote(&CreateNoteRequest{
		Text:         text,
		FileIDs:      []string{uploadedFile.ID},
		OriginalNote: note,
	}); err != nil {
		return errors.Wrap(err, "Failed to CreateNote")
	}

	log.Printf("Successfully processed amesh command for %s", location.PlaceName)
	return nil
}

// apiRequest MisskeyAPIリクエストを送信
func (bot *Bot) apiRequest(endpoint string, data interface{}) (*http.Response, error) {
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
		return nil, errors.Wrap(err, "Failed to libHttp.NewRequest")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", bot.UserAgent)

	resp, err := bot.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to Do")
	}

	return resp, nil
}

// ParseAmeshCommand ameshコマンドを解析
func ParseAmeshCommand(text string) ParseResult {
	// メンションを除去
	text = strings.TrimSpace(text)

	// @username を削除
	words := strings.Fields(text)
	var cleanWords []string
	for _, word := range words {
		if !strings.HasPrefix(word, "@") {
			cleanWords = append(cleanWords, word)
		}
	}
	text = strings.Join(cleanWords, " ")

	// ameshコマンドかチェック
	if strings.HasPrefix(text, "amesh ") {
		place := strings.TrimPrefix(text, "amesh ")
		return ParseResult{
			Place:   strings.TrimSpace(place),
			IsAmesh: true,
		}
	}

	if text == "amesh" {
		return ParseResult{
			Place:   "東京", // デフォルトの場所
			IsAmesh: true,
		}
	}

	return ParseResult{
		Place:   "",
		IsAmesh: false,
	}
}

// checkStatusAndDecodeJSON ステータスコードをチェックしJSONをデコードする共通処理
func checkStatusAndDecodeJSON(resp *http.Response, target interface{}) error {
	if resp.StatusCode != 200 {
		if err := resp.Body.Close(); err != nil {
			return errors.Wrap(err, "Failed to Close")
		}
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	defer func(body io.ReadCloser) {
		if closeErr := body.Close(); closeErr != nil {
			panic(errors.Wrap(closeErr, "Failed to Close"))
		}
	}(resp.Body)

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return errors.Wrap(err, "Failed to json.NewDecoder")
	}

	return nil
}
