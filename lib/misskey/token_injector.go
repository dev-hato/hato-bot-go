package misskey

import "net/http"

// tokenInjectingTransport Misskey APIトークンを接続先URLへ残さないための http.RoundTripper 実装。
// coder/websocket の Dial へはトークン抜きのURLを渡し、実際に送信するリクエストの複製にだけ i クエリを付与する。
// こうすると通信にはトークンが乗るが、Dial失敗時のエラーが参照するURLオブジェクトにはトークンが残らない。
type tokenInjectingTransport struct {
	base  http.RoundTripper // 実際の送信を担うトランスポート
	host  string            // トークンを付与してよい接続先ホスト
	token string            // Misskey APIトークン
}

// RoundTrip 送信リクエストの複製にだけトークンを付与して送信する
func (t *tokenInjectingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// リダイレクト等で別ホストへ飛んだ場合はトークンを付与しない
	if req.URL.Host != t.host {
		return t.base.RoundTrip(req)
	}

	// http.Clientが保持する元リクエスト（エラーのURLに使われる）は書き換えず、複製にだけ付与する
	forwarded := req.Clone(req.Context())

	query := forwarded.URL.Query()
	query.Set("i", t.token)
	forwarded.URL.RawQuery = query.Encode()

	return t.base.RoundTrip(forwarded)
}
