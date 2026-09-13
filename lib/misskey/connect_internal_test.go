package misskey

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"hato-bot-go/lib/httpclient"
)

// wantChannelName connectが購読すべきMisskeyストリーミングのチャンネル名
const wantChannelName = "main"

// connectFrame connectがサーバーへ送る購読メッセージ
type connectFrame struct {
	Type string            `json:"type"`
	Body map[string]string `json:"body"`
}

// startConnectTestServer connectフレームを1つ受け取ってチャネルへ流すテスト用WebSocketサーバーを起動する
func startConnectTestServer(t *testing.T) (wsURL *url.URL, received <-chan connectFrame) {
	t.Helper()

	got := make(chan connectFrame, 1)
	wsURL = StartWSTestServer(t, func(ctx context.Context, _ *http.Request, conn *websocket.Conn) {
		var frame connectFrame

		if readErr := wsjson.Read(ctx, conn, &frame); readErr != nil {
			t.Errorf("wsjson.Read() error = %v", readErr)
			return
		}

		got <- frame
		<-ctx.Done()
	})

	return wsURL, got
}

// newConnectTestBot HTTPクライアントをモックしたテスト用Botを返す
func newConnectTestBot() *Bot {
	return NewBotWithClient(&BotSetting{
		Domain: "example.com",
		Token:  "token",
		Client: httpclient.NewMockHTTPClient(http.StatusOK, ""),
	})
}

func TestConnect(t *testing.T) {
	tests := []struct {
		name          string
		withStaleConn bool // 事前に接続を確立しておくか（張り替え動作の検証）
	}{
		{name: "新規接続でmainチャンネルを購読する", withStaleConn: false},
		{name: "既存接続があっても張り替えてmainチャンネルを購読する", withStaleConn: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wsURL, received := startConnectTestServer(t)
			bot := newConnectTestBot()
			ctx := t.Context()

			// 事前接続（張り替え前の古い接続）を用意する
			var staleConn *websocket.Conn

			if tt.withStaleConn {
				staleURL, _ := startConnectTestServer(t)

				if err := bot.connect(ctx, &connectParams{WSURL: staleURL}); err != nil {
					t.Fatalf("事前のconnect() error = %v", err)
				}

				staleConn = bot.WSConn
			}

			if err := bot.connect(ctx, &connectParams{WSURL: wsURL}); err != nil {
				t.Fatalf("connect() error = %v", err)
			}

			t.Cleanup(func() {
				if bot.WSConn == nil {
					return
				}

				if closeErr := bot.WSConn.CloseNow(); closeErr != nil {
					t.Logf("client WSConn.CloseNow() error = %v", closeErr)
				}
			})

			if bot.WSConn == nil {
				t.Fatal("connect() 後に WSConn が nil")
			}

			// サーバーが受け取った購読フレームの中身を検証する
			var frame connectFrame
			select {
			case frame = <-received:
			case <-time.After(5 * time.Second):
				t.Fatal("connectフレームがサーバーへ届かなかった")
			}

			frameChecks := []struct {
				field string
				got   string
				want  string
			}{
				{"type", frame.Type, "connect"},
				{"body.channel", frame.Body["channel"], wantChannelName},
				{"body.id", frame.Body["id"], wantChannelName},
			}

			for _, c := range frameChecks {
				if c.got != c.want {
					t.Errorf("connectフレーム %s = %q, want %q", c.field, c.got, c.want)
				}
			}

			// 事前接続がある場合は、張り替えられて古い接続が閉じられていること
			if tt.withStaleConn {
				if bot.WSConn == staleConn {
					t.Error("connect() を再実行しても WSConn が張り替えられていない")
				}

				if err := wsjson.Write(ctx, staleConn, map[string]string{"type": "ping"}); err == nil {
					t.Error("古い接続への書き込みが成功した（閉じられていない）")
				}
			}
		})
	}
}

// TestListenAfterFailedReconnect 接続成功後に再接続へ失敗し、WSConnがnilのままListenを呼んでもpanicせずエラーを返すことを検証する。
// cmd/misskey_bot の再接続ループはConnect失敗後もループ先頭でListenを呼び直すため、
// nil接続を参照してwsjson.Readがpanicする異常系を再現する。
func TestListenAfterFailedReconnect(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// まず一度は接続に成功させる
	wsURL, _ := startConnectTestServer(t)
	bot := newConnectTestBot()

	if err := bot.connect(ctx, &connectParams{WSURL: wsURL}); err != nil {
		t.Fatalf("初回のconnect() error = %v", err)
	}

	if bot.WSConn == nil {
		t.Fatal("初回connect()後にWSConnがnil")
	}

	// WebSocketへアップグレードせず400を返すサーバーを相手に、再接続を確実に失敗させる
	httpOnly := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(httpOnly.Close)

	badURL := &url.URL{Scheme: "ws", Host: httpOnly.Listener.Addr().String()}

	if err := bot.connect(ctx, &connectParams{WSURL: badURL}); err == nil {
		t.Fatal("再接続が成功してしまった（失敗を期待）")
	}

	// 再接続失敗後、WSConnはnilへ戻っている（この異常系が今回の再現条件）
	if bot.WSConn != nil {
		t.Fatal("再接続失敗後にWSConnがnilになっていない")
	}

	// nil接続のままListenを呼んでもpanicせず、エラーを返すこと
	if err := bot.Listen(ctx, func(*Note) {}); err == nil {
		t.Error("Listen() error = nil, want non-nil（nil接続）")
	}
}

// startTokenCapturingWSServer ハンドシェイクを受理し、リクエストの i クエリをチャネルへ流すテスト用サーバーを起動する
func startTokenCapturingWSServer(t *testing.T) (wsURL *url.URL, gotToken <-chan string) {
	t.Helper()

	tokenCh := make(chan string, 1)
	wsURL = StartWSTestServer(t, func(ctx context.Context, r *http.Request, _ *websocket.Conn) {
		tokenCh <- r.URL.Query().Get("i")
		<-ctx.Done()
	})

	return wsURL, tokenCh
}

// tokenSecret 全テストで使うダミーのAPIトークン
const tokenSecret = "review-dummy-secret-token"

// TestConnectSendsTokenOnlyOnWire connectがトークンを接続先URLへ載せず、
// カスタムTransport経由で送信リクエストにだけ i クエリとして付与し、
// かつハンドシェイクが成立することを検証する（正常系）。
func TestConnectSendsTokenOnlyOnWire(t *testing.T) {
	t.Parallel()

	wsURL, gotToken := startTokenCapturingWSServer(t)
	bot := newConnectTestBot()

	if err := bot.connect(t.Context(), &connectParams{WSURL: wsURL, Token: tokenSecret}); err != nil {
		t.Fatalf("connect() error = %v", err)
	}

	t.Cleanup(func() {
		if bot.WSConn == nil {
			return
		}

		if err := bot.WSConn.CloseNow(); err != nil {
			t.Fatal(err)
		}
	})

	if bot.WSConn == nil {
		t.Fatal("connect() 後に WSConn が nil")
	}

	select {
	case got := <-gotToken:
		if got != tokenSecret {
			t.Errorf("サーバーが受け取った i = %q, want %q", got, tokenSecret)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("サーバーが i クエリを受け取らなかった")
	}
}

// TestConnectDialErrorHasNoToken Dial失敗時のエラー文字列にAPIトークンが残らないことを、
// connect / Connect の両経路について検証する（異常系。main の起動時・再接続時ログへ漏れない）。
func TestConnectDialErrorHasNoToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// dial はキャンセル済みcontextを受け取り、Dialを失敗させて返ったエラーを返す
		dial func(ctx context.Context) error
	}{
		{
			name: "connect経由",
			dial: func(ctx context.Context) error {
				return newConnectTestBot().connect(ctx, &connectParams{
					WSURL: &url.URL{Scheme: "wss", Host: "example.com", Path: streamingPath},
					Token: tokenSecret,
				})
			},
		},
		{
			name: "Connect経由",
			dial: func(ctx context.Context) error {
				return NewBotWithClient(&BotSetting{
					Domain: "example.com",
					Token:  tokenSecret,
					Client: httpclient.NewMockHTTPClient(http.StatusOK, ""),
				}).Connect(ctx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// キャンセル済みcontextでDialを即座に失敗させる
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := tt.dial(ctx)
			if err == nil {
				t.Fatal("error = nil, want non-nil")
			}

			if strings.Contains(err.Error(), tokenSecret) {
				t.Errorf("エラーにトークンが露出している: %v", err)
			}
		})
	}
}
