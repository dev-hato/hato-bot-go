package misskey

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
)

// StartWSTestServer websocket.Acceptで接続を受け付けるテスト用サーバーを起動し、ws://スキームの接続先URLを返す。
// handle は接続ごとに呼ばれ、リクエストのコンテキストとWebSocket接続を受け取る。
// package misskey 内・外の双方のテストから使えるようエクスポートしている。
func StartWSTestServer(t *testing.T, handle func(ctx context.Context, conn *websocket.Conn)) string {
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

		handle(r.Context(), conn)
	}))
	t.Cleanup(srv.Close)

	// httptestサーバーは平文HTTPのため、ws://スキームで接続する
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/streaming"
}
