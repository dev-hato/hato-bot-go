package misskey

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"hato-bot-go/lib"
	"hato-bot-go/lib/amesh"
	"hato-bot-go/lib/httpclient"
)

// Bot Misskeyボットクライアント
type Bot struct {
	BotSetting *BotSetting
	UserAgent  string
	WSConn     *websocket.Conn
}

// CreateNote ノートを作成
func (bot *Bot) CreateNote(ctx context.Context, params *CreateNoteParams) (err error) {
	if params == nil || params.OriginalNote == nil {
		return lib.ErrParamsNil
	}

	// noteから必要な情報を取得
	visibility := params.OriginalNote.Visibility
	replyID := params.OriginalNote.ID

	// 公開範囲がpublicならばhomeにする
	if visibility == "public" {
		visibility = "home"
	}

	data := map[string]any{
		"text":       params.Text,
		"visibility": visibility,
	}

	if replyID != "" {
		data["replyId"] = replyID
	}

	if 0 < len(params.FileIDs) {
		data["fileIds"] = params.FileIDs
	}

	// 元の投稿がCWされていた場合、それに合わせてCW投稿する
	if params.OriginalNote.CW != nil {
		data["cw"] = "隠すっぽ！"
	}

	// jscpd:ignore-start
	resp, err := bot.apiRequest(ctx, "notes/create", data)
	if err != nil {
		return errors.Wrap(err, "Failed to apiRequest")
	}
	defer func(body io.ReadCloser) {
		if closeErr := body.Close(); closeErr != nil {
			err = errors.Join(err, errors.Wrap(closeErr, "Failed to Close"))
		}
	}(resp.Body)
	// jscpd:ignore-end

	var result struct {
		CreatedNote Note `json:"createdNote"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return errors.Wrap(err, "Failed to json.NewDecoder")
	}

	return nil
}

// UploadFile ファイルをアップロード
func (bot *Bot) UploadFile(ctx context.Context, reader io.Reader, fileName string) (file *File, err error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	defer func(writer *multipart.Writer) {
		if closeErr := writer.Close(); closeErr != nil {
			err = errors.Join(err, errors.Wrap(closeErr, "Failed to Close"))
		}
	}(writer)

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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to http.NewRequestWithContext")
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// jscpd:ignore-start
	resp, err := httpclient.ExecuteHTTPRequest(bot.BotSetting.Client, req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to executeHTTPRequest")
	}
	defer func(body io.ReadCloser) {
		if closeErr := body.Close(); closeErr != nil {
			err = errors.Join(err, errors.Wrap(closeErr, "Failed to Close"))
		}
	}(resp.Body)
	// jscpd:ignore-end

	var uploadedFile File
	if err = json.NewDecoder(resp.Body).Decode(&uploadedFile); err != nil {
		return nil, errors.Wrap(err, "Failed to json.NewDecoder")
	}

	return &uploadedFile, nil
}

// AddReaction リアクションを追加
func (bot *Bot) AddReaction(ctx context.Context, noteID, reaction string) (err error) {
	data := map[string]any{
		"noteId":   noteID,
		"reaction": reaction,
	}

	// jscpd:ignore-start
	resp, err := bot.apiRequest(ctx, "notes/reactions/create", data)
	if err != nil {
		return errors.Wrap(err, "Failed to apiRequest")
	}
	defer func(body io.ReadCloser) {
		if closeErr := body.Close(); closeErr != nil {
			err = errors.Join(err, errors.Wrap(closeErr, "Failed to Close"))
		}
	}(resp.Body)
	// jscpd:ignore-end

	return nil
}

// ProcessAmeshCommand ameshコマンドを処理
func (bot *Bot) ProcessAmeshCommand(ctx context.Context, params *ProcessAmeshCommandParams) error {
	if params == nil || params.Note == nil {
		return lib.ErrParamsNil
	}
	if params.YahooAPIToken == "" {
		return lib.ErrParamsEmptyString
	}

	// 処理中リアクションを追加
	if err := bot.AddReaction(ctx, params.Note.ID, "👀"); err != nil {
		return errors.Wrap(err, "Failed to AddReaction")
	}

	// 位置を解析
	location, err := amesh.ParseLocationWithLog(ctx, params.Place, params.YahooAPIToken)
	if err != nil {
		return errors.Wrap(err, "Failed to amesh.ParseLocationWithLog")
	}

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
	if err := bot.CreateNote(ctx, &CreateNoteParams{
		Text:         text,
		FileIDs:      []string{uploadedFile.ID},
		OriginalNote: params.Note,
	}); err != nil {
		return errors.Wrap(err, "Failed to CreateNote")
	}

	log.Printf("Successfully processed amesh command for %s", location.PlaceName)
	return nil
}

// Connect WebSocket接続を確立
func (bot *Bot) Connect(ctx context.Context) error {
	wsURL := fmt.Sprintf("wss://%s/streaming?i=%s", bot.BotSetting.Domain, bot.BotSetting.Token)
	return errors.Wrap(bot.connect(ctx, wsURL), "Failed to connect")
}

// connect 指定したURLへWebSocket接続を確立する
// テストから平文WebSocketサーバーへ接続できるようURLを引数で受け取る内部メソッド
func (bot *Bot) connect(ctx context.Context, wsURL string) (err error) {
	// 古い接続が残っている場合はリソースを解放する
	if bot.WSConn != nil {
		if closeErr := bot.WSConn.CloseNow(); closeErr != nil {
			log.Printf("Failed to WSConn.CloseNow(): %v", closeErr)
		}

		bot.WSConn = nil
	}

	// ハンドシェイクのタイムアウトを10秒に設定する
	// このコンテキストはDialの間だけ使い、接続確立後の読み書きには影響しない
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, resp, err := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"User-Agent": []string{bot.UserAgent},
		},
	})
	// Dialの成否に関わらず、ハンドシェイク応答のBodyが存在すれば必ずCloseする。
	// ただしClose失敗を戻り値へ合成するのはconnectが既にエラーの場合のみとし、
	// 接続に成功しているのにClose失敗で戻り値を汚染しないようにする。
	if resp != nil && resp.Body != nil {
		defer func(body io.ReadCloser) {
			closeErr := body.Close()
			switch {
			case closeErr == nil:
				return
			case err != nil:
				err = errors.Join(err, errors.Wrap(closeErr, "Failed to Close"))
			default:
				log.Printf("Failed to Close: %v", closeErr)
			}
		}(resp.Body)
	}
	if err != nil {
		return errors.Wrap(err, "Failed to Dial")
	}

	// Misskeyのストリーミングメッセージは大きくなり得るため、読み取りサイズの上限を撤廃する
	conn.SetReadLimit(-1)

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

	if err := wsjson.Write(ctx, bot.WSConn, connectMsg); err != nil {
		return errors.Wrap(err, "Failed to wsjson.Write")
	}

	log.Printf("Connected to Misskey WebSocket: %s", bot.BotSetting.Domain)
	return nil
}

// Listen WebSocketメッセージを監視
func (bot *Bot) Listen(ctx context.Context, messageHandler func(note *Note)) error {
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
		if err := wsjson.Read(ctx, bot.WSConn, &msg); err != nil {
			return errors.Wrap(err, "Failed to wsjson.Read")
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
func (bot *Bot) apiRequest(ctx context.Context, endpoint string, data map[string]any) (*http.Response, error) {
	// データにトークンを追加
	payload := map[string]any{
		"i": bot.BotSetting.Token,
	}

	maps.Copy(payload, data)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to json.Marshal")
	}

	url := fmt.Sprintf("https://%s/api/%s", bot.BotSetting.Domain, endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to http.NewRequestWithContext")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.ExecuteHTTPRequest(bot.BotSetting.Client, req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to executeHTTPRequest")
	}

	return resp, nil
}
