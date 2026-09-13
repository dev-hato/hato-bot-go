package misskey

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/coder/websocket"
)

// StartWSTestServer websocket.Acceptで接続を受け付けるテスト用サーバーを起動し、ws://スキームの接続先URLを返す。
// handle は接続ごとに呼ばれ、テスト終了時にキャンセルされるコンテキストとハンドシェイク時のリクエスト、WebSocket接続を受け取る。
// リクエストはクエリパラメータ検証など、ハンドシェイク前の情報を確認したいテストのために渡している。
// package misskey 内・外の双方のテストから使えるようエクスポートしている。
func StartWSTestServer(t *testing.T, handle func(ctx context.Context, r *http.Request, conn *websocket.Conn)) *url.URL {
	t.Helper()

	// Hijack後の r.Context() はハンドラーが戻るまでキャンセルされず、 handle が <-ctx.Done() で待つとハンドラーとgoroutineが残る。
	// テスト終了時に解放できるようcleanupでキャンセルする専用contextを渡す。
	ctx, cancel := context.WithCancel(context.Background())

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

		handle(ctx, r, conn)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(cancel) // テスト終了時にキャンセルし、<-ctx.Done() で待つハンドラーを解放する

	// httptestサーバーは平文HTTPのため、ws://スキームで接続する
	return &url.URL{Scheme: "ws", Host: srv.Listener.Addr().String(), Path: streamingPath}
}
