package misskey

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gorilla/websocket"

	"hato-bot-go/lib"
	"hato-bot-go/lib/amesh"
	libHttp "hato-bot-go/lib/http"
)

// エラー定数
var (
	ErrParamsEmptyString = errors.New("params cannot be empty string")
)

// Bot Misskeyボットクライアント
type Bot struct {
	BotSetting *BotSetting
	UserAgent  string
	WSConn     *websocket.Conn
}

// CreateNote ノートを作成
func (bot *Bot) CreateNote(ctx context.Context, req *CreateNoteRequest) error {
	if req == nil || req.OriginalNote == nil {
		return lib.ErrParamsNil
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

	resp, err := bot.apiRequest(ctx, "notes/create", data)
	if err != nil {
		return errors.Wrap(err, "Failed to apiRequest")
	}

	var result struct {
		CreatedNote Note `json:"createdNote"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return errors.Wrap(err, "Failed to json.NewDecoder")
	}

	return nil
}

// UploadFile ファイルをアップロード
func (bot *Bot) UploadFile(ctx context.Context, reader io.Reader, fileName string) (*File, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// トークンフィールドを追加
	if writeErr := writer.WriteField("i", bot.BotSetting.Token); writeErr != nil {
		return nil, errors.Wrap(writeErr, "Failed to WriteField")
	}

	// ファイルフィールドを追加
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to CreateFormFile")
	}

	if _, copyErr := io.Copy(part, reader); copyErr != nil {
		return nil, errors.Wrap(copyErr, "Failed to io.Copy")
	}

	if closeErr := writer.Close(); closeErr != nil {
		return nil, errors.Wrap(closeErr, "Failed to Close")
	}

	url := fmt.Sprintf("https://%s/api/drive/files/create", bot.BotSetting.Domain)
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to http.NewRequestWithContext")
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", bot.UserAgent)

	resp, err := libHttp.ExecuteHTTPRequest(bot.BotSetting.Client, req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to executeHTTPRequest")
	}

	var uploadedFile File
	if err = json.NewDecoder(resp.Body).Decode(&uploadedFile); err != nil {
		return nil, errors.Wrap(err, "Failed to json.NewDecoder")
	}

	return &uploadedFile, nil
}

// AddReaction リアクションを追加
func (bot *Bot) AddReaction(ctx context.Context, noteID, reaction string) (err error) {
	data := map[string]string{
		"noteId":   noteID,
		"reaction": reaction,
	}

	if _, err := bot.apiRequest(ctx, "notes/reactions/create", data); err != nil {
		return errors.Wrap(err, "Failed to apiRequest")
	}

	return nil
}

// ProcessAmeshCommand ameshコマンドを処理
func (bot *Bot) ProcessAmeshCommand(ctx context.Context, req *ProcessAmeshCommandRequest) error {
	if req == nil || req.Note == nil {
		return lib.ErrParamsNil
	}
	if req.YahooAPIToken == "" {
		return ErrParamsEmptyString
	}

	// 処理中リアクションを追加
	if err := bot.AddReaction(ctx, req.Note.ID, "👀"); err != nil {
		return errors.Wrap(err, "Failed to AddReaction")
	}

	// 位置を解析
	location, err := amesh.ParseLocation(ctx, req.Place, req.YahooAPIToken)
	if err != nil {
		return errors.Wrap(err, "Failed to amesh.ParseLocation")
	}

	fmt.Printf("Generating amesh image for %s (%.4f, %.4f)\n", location.PlaceName, location.Lat, location.Lng)

	// 画像をメモリ上に作成
	imageReader, err := amesh.CreateImageReader(ctx, location)
	if err != nil {
		return errors.Wrap(err, "Failed to amesh.CreateImageReader")
	}

	// ファイル名を生成
	fileName := amesh.GenerateFileName(location)

	// Misskeyにメモリから直接アップロード
	uploadedFile, err := bot.UploadFile(ctx, imageReader, fileName)
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
	if err := bot.CreateNote(ctx, &CreateNoteRequest{
		Text:         text,
		FileIDs:      []string{uploadedFile.ID},
		OriginalNote: req.Note,
	}); err != nil {
		return errors.Wrap(err, "Failed to CreateNote")
	}

	log.Printf("Successfully processed amesh command for %s", location.PlaceName)
	return nil
}

// Connect WebSocket接続を確立
func (bot *Bot) Connect() error {
	wsURL := fmt.Sprintf("wss://%s/streaming?i=%s", bot.BotSetting.Domain, bot.BotSetting.Token)

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(wsURL, http.Header{
		"User-Agent": []string{bot.UserAgent},
	})
	if err != nil {
		return errors.Wrap(err, "Failed to Dial")
	}

	bot.WSConn = conn

	// メインチャンネルに接続
	connectMsg := struct {
		Type string            `json:"type"`
		Body map[string]string `json:"body,omitempty"`
	}{
		Type: "connect",
		Body: map[string]string{
			"channel": "main",
			"id":      "main",
		},
	}

	if err := bot.WSConn.WriteJSON(connectMsg); err != nil {
		return errors.Wrap(err, "Failed to WriteJSON")
	}

	log.Printf("Connected to Misskey WebSocket: %s", bot.BotSetting.Domain)
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
		if err := bot.WSConn.ReadJSON(&msg); err != nil {
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

// apiRequest MisskeyAPIリクエストを送信
func (bot *Bot) apiRequest(ctx context.Context, endpoint string, data interface{}) (*http.Response, error) {
	// データにトークンを追加
	payload := map[string]interface{}{
		"i": bot.BotSetting.Token,
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

	url := fmt.Sprintf("https://%s/api/%s", bot.BotSetting.Domain, endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to http.NewRequestWithContext")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", bot.UserAgent)

	resp, err := libHttp.ExecuteHTTPRequest(bot.BotSetting.Client, req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to executeHTTPRequest")
	}

	return resp, nil
}
