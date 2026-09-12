package misskey

import (
	"net/http"

	"github.com/cockroachdb/errors"
)

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
	// 別ホスト宛にはトークンを付与しない。
	// 呼び出し側のhttp.ClientはCheckRedirectでリダイレクト自体を拒否しているため、本来ここには来ない想定だが、
	// 設定変更時の保険として残している
	if req.URL.Host != t.host {
		return t.base.RoundTrip(req)
	}

	// http.Clientが保持する元リクエスト（エラーのURLに使われる）は書き換えず、複製にだけ付与する
	forwarded := req.Clone(req.Context())

	query := forwarded.URL.Query()
	query.Set("i", t.token)
	forwarded.URL.RawQuery = query.Encode()

	resp, err := t.base.RoundTrip(forwarded)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to RoundTrip")
	}

	// 実際に送信したforwardedをRoundTripperがResponse.Requestへそのまま残すため、トークン抜きの元リクエストへ戻す。
	// net/httpはリダイレクト先URL解析の失敗時などresp.Request.URLをエラーへ転用することがあり、そこからのトークン漏れを防ぐ
	resp.Request = req

	return resp, nil
}
