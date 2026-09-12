package misskey

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestConnectRedirectErrorHasNoToken クエリを保持するリダイレクトの後で接続が失敗しても、
// 呼び出し元がログに出力するエラーへAPIトークンが露出しないことを検証する。
func TestConnectRedirectErrorHasNoToken(t *testing.T) {
	t.Parallel()

	initialTokens := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(streamingPath, func(w http.ResponseWriter, r *http.Request) {
		select {
		case initialTokens <- r.URL.Query().Get("i"):
		default:
		}

		// 末尾のスラッシュを補う際、受信した認証クエリもLocationへ引き継ぐ。
		http.Redirect(w, r, streamingPath+"/?"+r.URL.RawQuery, http.StatusMovedPermanently)
	})
	mux.HandleFunc(streamingPath+"/", func(http.ResponseWriter, *http.Request) {
		// HTTPサーバーに応答を中断させ、WebSocketへのアップグレード前に接続を閉じる。
		panic(http.ErrAbortHandler)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	err := newConnectTestBot().connect(t.Context(), &connectParams{
		WSURL: &url.URL{Scheme: "ws", Host: srv.Listener.Addr().String(), Path: streamingPath},
		Token: tokenSecret,
	})

	// 最初の接続自体が失敗した場合や、認証トークンを送らなくなった場合の偽陽性を防ぐ。
	select {
	case got := <-initialTokens:
		if got != tokenSecret {
			t.Fatalf("最初のリクエストの i = %q, want %q", got, tokenSecret)
		}
	default:
		t.Fatalf("最初のリクエストがサーバーへ届かなかった: %v", err)
	}

	if err == nil {
		t.Fatal("connect() error = nil, want non-nil")
	}

	// リダイレクトを拒否する修正も許容し、追従やEOFそのものは必須条件にしない。
	if strings.Contains(err.Error(), tokenSecret) {
		t.Errorf("リダイレクト後の接続エラーにトークンが露出している: %v", err)
	}
}
