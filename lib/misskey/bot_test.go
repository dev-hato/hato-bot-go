package misskey_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"hato-bot-go/lib"
	"hato-bot-go/lib/httpclient"
	"hato-bot-go/lib/misskey"
)

func TestAddReaction(t *testing.T) {
	tests := []struct {
		name        string
		noteID      string
		reaction    string
		statusCode  int
		expectError error
	}{
		{
			name:        "正常なリアクション追加",
			noteID:      "note123",
			reaction:    "👍",
			statusCode:  http.StatusNoContent,
			expectError: nil,
		},
		{
			name:        "APIエラー応答",
			noteID:      "note456",
			reaction:    "❤️",
			statusCode:  http.StatusBadRequest,
			expectError: httpclient.ErrHTTPRequestError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runSimpleBotTest(t, &runSimpleBotTestParams{
				StatusCode: tt.statusCode,
				TestFunc: func(bot *misskey.Bot) error {
					return bot.AddReaction(t.Context(), tt.noteID, tt.reaction)
				},
				ExpectError: tt.expectError,
				TestName:    "AddReaction()",
			})
		})
	}
}

func TestCreateNote(t *testing.T) {
	tests := []struct {
		name         string
		params       *misskey.CreateNoteParams
		statusCode   int
		responseBody string
		expectError  error
	}{
		{
			name:         "nilリクエスト",
			params:       nil,
			statusCode:   http.StatusOK,
			responseBody: `{"createdNote":{"id":"created123"}}`,
			expectError:  lib.ErrParamsNil,
		},
		{
			name: "nil OriginalNote",
			params: &misskey.CreateNoteParams{
				Text:         "test",
				OriginalNote: nil,
			},
			statusCode:   http.StatusOK,
			responseBody: `{"createdNote":{"id":"created123"}}`,
			expectError:  lib.ErrParamsNil,
		},
		{
			name: "有効なリクエスト",
			params: &misskey.CreateNoteParams{
				Text: "test note",
				OriginalNote: &misskey.Note{
					ID:         "original123",
					Visibility: "home",
				},
			},
			statusCode:   http.StatusOK,
			responseBody: `{"createdNote":{"id":"created123"}}`,
			expectError:  nil,
		},
		// jscpd:ignore-start
		{
			name: "APIエラー応答",
			params: &misskey.CreateNoteParams{
				Text: "test note",
				OriginalNote: &misskey.Note{
					ID:         "original123",
					Visibility: "home",
				},
			},
			statusCode:   http.StatusBadRequest,
			responseBody: `{"error":"bad request"}`,
			expectError:  httpclient.ErrHTTPRequestError,
		},
		// jscpd:ignore-end
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runBotTest(t, &runBotTestParams{
				StatusCode:   tt.statusCode,
				ResponseBody: tt.responseBody,
				TestFunc: func(bot *misskey.Bot) error {
					return bot.CreateNote(t.Context(), tt.params)
				},
				ExpectError: tt.expectError,
				TestName:    "CreateNote()",
			})
		})
	}
}

func TestUploadFile(t *testing.T) {
	tests := []struct {
		name         string
		fileName     string
		readerData   string
		statusCode   int
		responseBody string
		expectError  error
	}{
		{
			name:         "成功したファイルアップロード",
			fileName:     "test.txt",
			readerData:   "test file content",
			statusCode:   http.StatusOK,
			responseBody: `{"id":"file123","name":"test.txt","url":"https://example.com/file123"}`,
			expectError:  nil,
		},
		// jscpd:ignore-start
		{
			name:         "APIエラー応答",
			fileName:     "test.txt",
			readerData:   "test content",
			statusCode:   http.StatusBadRequest,
			responseBody: `{"error":"bad request"}`,
			expectError:  httpclient.ErrHTTPRequestError,
		},
		// jscpd:ignore-end
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Helper()
			mockClient := httpclient.NewMockHTTPClient(tt.statusCode, tt.responseBody)
			bot := misskey.NewBotWithClient(&misskey.BotSetting{
				Domain: "example.com",
				Token:  "token",
				Client: mockClient,
			})

			reader := strings.NewReader(tt.readerData)
			if _, err := bot.UploadFile(t.Context(), reader, tt.fileName); !errors.Is(err, tt.expectError) {
				t.Errorf("UploadFile() error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}

// newTestWSBot テスト用WebSocketサーバーへ接続済みのBotを返す
// serverFn はサーバー側の接続ハンドラーで、受け取ったconnとリクエストコンテキストを使ってメッセージを送受信する
func newTestWSBot(t *testing.T, serverFn func(ctx context.Context, conn *websocket.Conn)) *misskey.Bot {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("websocket.Accept() error = %v", err)
			return
		}
		defer func() {
			if closeErr := conn.CloseNow(); closeErr != nil {
				t.Logf("server conn.CloseNow() error = %v", closeErr)
			}
		}()

		serverFn(r.Context(), conn)
	}))
	t.Cleanup(srv.Close)

	bot := misskey.NewBotWithClient(&misskey.BotSetting{
		Domain: "example.com",
		Token:  "token",
		Client: httpclient.NewMockHTTPClient(http.StatusOK, ""),
	})

	// httptestサーバーは平文HTTPのため、ws://スキームで直接ダイヤルしてWSConnへ注入する
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/streaming"

	conn, resp, err := websocket.Dial(t.Context(), wsURL, nil)
	if resp != nil && resp.Body != nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("handshake response body Close() error = %v", closeErr)
		}
	}
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}

	bot.WSConn = conn
	t.Cleanup(func() {
		if closeErr := conn.CloseNow(); closeErr != nil {
			t.Logf("client conn.CloseNow() error = %v", closeErr)
		}
	})

	return bot
}

// wsChannelMention mentionイベントのストリーミングフレームを組み立てる
func wsChannelMention(noteID, text, username string) map[string]any {
	return map[string]any{
		"type": "channel",
		"body": map[string]any{
			"type": "mention",
			"body": map[string]any{
				"id":   noteID,
				"text": text,
				"user": map[string]any{"username": username},
			},
		},
	}
}

// sentinelNoteID これを受け取ったら直前までのフレームが処理済みだと判断するための番兵ノートID
const sentinelNoteID = "__sentinel__"

func TestListen(t *testing.T) {
	// mention以外のイベント（typeがmentionではないchannelイベント）
	nonMention := map[string]any{
		"type": "channel",
		"body": map[string]any{"type": "reply", "body": map[string]any{"id": "note000"}},
	}
	// channel以外のイベント
	nonChannel := map[string]any{
		"type": "emojiUpdated",
		"body": map[string]any{},
	}

	tests := []struct {
		name        string
		frames      []map[string]any // サーバーが順に送信するフレーム
		wantNoteIDs []string         // ハンドラーへ渡るべきノートID（順序込み、番兵は除く）
	}{
		{
			name:        "mentionイベントはハンドラーへ渡る",
			frames:      []map[string]any{wsChannelMention("note123", "amesh 東京", "alice")},
			wantNoteIDs: []string{"note123"},
		},
		{
			name:        "reply(mention以外)は無視される",
			frames:      []map[string]any{nonMention},
			wantNoteIDs: nil,
		},
		{
			name:        "channel以外のtypeは無視される",
			frames:      []map[string]any{nonChannel},
			wantNoteIDs: nil,
		},
		{
			name: "無関係なイベントが混ざってもmentionだけ拾う",
			frames: []map[string]any{
				nonChannel,
				nonMention,
				wsChannelMention("note123", "amesh 大阪", "bob"),
				nonMention,
			},
			wantNoteIDs: []string{"note123"},
		},
		{
			name: "複数のmentionを送信順に受け取る",
			frames: []map[string]any{
				wsChannelMention("noteA", "amesh A", "a"),
				wsChannelMention("noteB", "amesh B", "b"),
			},
			wantNoteIDs: []string{"noteA", "noteB"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			frames := tt.frames
			bot := newTestWSBot(t, func(ctx context.Context, conn *websocket.Conn) {
				for _, f := range frames {
					if err := wsjson.Write(ctx, conn, f); err != nil {
						return
					}
				}

				// 番兵mention: これが処理されれば直前までのフレームは処理済み
				if err := wsjson.Write(ctx, conn, wsChannelMention(sentinelNoteID, "", "")); err != nil {
					return
				}
				<-ctx.Done()
			})

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			var gotNoteIDs []string
			done := make(chan error, 1)
			go func() {
				done <- bot.Listen(ctx, func(note *misskey.Note) {
					if note.ID == sentinelNoteID {
						cancel() // 番兵を受け取ったらListenを終了させる
						return
					}

					gotNoteIDs = append(gotNoteIDs, note.ID)
				})
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("Listenが終了しなかった")
			}

			if !slices.Equal(gotNoteIDs, tt.wantNoteIDs) {
				t.Errorf("ハンドラーへ渡ったノートID = %v, want %v", gotNoteIDs, tt.wantNoteIDs)
			}
		})
	}
}

// TestListenMentionContent mentionのノート内容がそのままハンドラーへ渡ることを検証する
func TestListenMentionContent(t *testing.T) {
	t.Parallel()

	bot := newTestWSBot(t, func(ctx context.Context, conn *websocket.Conn) {
		if err := wsjson.Write(ctx, conn, wsChannelMention("note123", "amesh 東京", "alice")); err != nil {
			return
		}
		<-ctx.Done()
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	got := make(chan *misskey.Note, 1)
	listenDone := make(chan error, 1)
	go func() {
		listenDone <- bot.Listen(ctx, func(note *misskey.Note) {
			got <- note
			cancel()
		})
	}()

	var note *misskey.Note

	select {
	case note = <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("mentionイベントがハンドラーへ届かなかった")
	}

	// コンテキストキャンセル後、Listenはエラーを返して終了する
	select {
	case err := <-listenDone:
		if err == nil {
			t.Error("Listen() error = nil, want non-nil after context cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Listenが終了しなかった")
	}

	fields := []struct {
		field string
		got   string
		want  string
	}{
		{"ID", note.ID, "note123"},
		{"Text", note.Text, "amesh 東京"},
		{"User.Username", note.User.Username, "alice"},
	}

	for _, f := range fields {
		if f.got != f.want {
			t.Errorf("note.%s = %q, want %q", f.field, f.got, f.want)
		}
	}
}

func TestListenInvalidArgs(t *testing.T) {
	tests := []struct {
		name    string
		handler func(note *misskey.Note)
	}{
		{name: "nilハンドラーはエラー", handler: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bot := misskey.NewBotWithClient(&misskey.BotSetting{
				Domain: "example.com",
				Token:  "token",
				Client: httpclient.NewMockHTTPClient(http.StatusOK, ""),
			})
			if err := bot.Listen(t.Context(), tt.handler); err == nil {
				t.Error("Listen() error = nil, want non-nil")
			}
		})
	}
}

func TestProcessAmeshCommand(t *testing.T) {
	tests := []struct {
		name        string
		params      *misskey.ProcessAmeshCommandParams
		expectError error
	}{
		{
			name:        "nilリクエスト",
			params:      nil,
			expectError: lib.ErrParamsNil,
		},
		{
			name: "nilノート",
			params: &misskey.ProcessAmeshCommandParams{
				Note:          nil,
				Place:         "東京",
				YahooAPIToken: "YahooAPIToken",
			},
			expectError: lib.ErrParamsNil,
		},
		{
			name: "Yahoo APIトークンが設定されていない",
			params: &misskey.ProcessAmeshCommandParams{
				Note: &misskey.Note{
					ID:         "note123",
					Visibility: "home",
				},
				Place: "東京",
			},
			expectError: lib.ErrParamsEmptyString, // Yahoo APIトークンが設定されていないためエラーが発生する
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runSimpleBotTest(t, &runSimpleBotTestParams{
				StatusCode: http.StatusNoContent,
				TestFunc: func(bot *misskey.Bot) error {
					return bot.ProcessAmeshCommand(t.Context(), tt.params)
				},
				ExpectError: tt.expectError,
				TestName:    "ProcessAmeshCommand()",
			})
		})
	}
}
