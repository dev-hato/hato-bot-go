package misskey

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc テスト用の http.RoundTripper スタブ
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// TestTokenInjectingTransportRoundTrip 送信リクエストの複製にだけトークンが付与され、
// 呼び出し元のリクエストURLは書き換わらず、Response.Requestもトークン抜きの元リクエストへ差し戻されることを検証する。
func TestTokenInjectingTransportRoundTrip(t *testing.T) {
	t.Parallel()

	const token = "secret-token-value"

	tests := []struct {
		name       string
		injectHost string // トークンを付与してよいホスト
		reqURL     string // RoundTripへ渡すリクエストのURL
		wantSentI  string // 実際に送信されたリクエストの i クエリ
	}{
		{
			name:       "ホスト一致なら複製へトークンを付与する",
			injectHost: "misskey.example",
			reqURL:     "https://misskey.example/streaming",
			wantSentI:  token,
		},
		{
			name:       "ホスト不一致ならトークンを付与しない",
			injectHost: "misskey.example",
			reqURL:     "https://other.example/streaming",
			wantSentI:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var sentI string

			stub := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				sentI = req.URL.Query().Get("i")

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			})

			transport := &tokenInjectingTransport{base: stub, host: tt.injectHost, token: token}

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, tt.reqURL, nil)
			if err != nil {
				t.Fatal(err)
			}

			resp, err := transport.RoundTrip(req)
			if err != nil {
				t.Fatalf("RoundTrip() error = %v", err)
			}

			if err := resp.Body.Close(); err != nil {
				t.Fatal(err)
			}

			if sentI != tt.wantSentI {
				t.Errorf("送信リクエストの i = %q, want %q", sentI, tt.wantSentI)
			}

			// 呼び出し元のリクエストURLへトークンが混入していないこと
			if got := req.URL.Query().Get("i"); got != "" {
				t.Errorf("元リクエストのURLにトークンが混入した: i = %q", got)
			}

			// net/httpの実装ではResponse.Requestに実際に送信したリクエスト（トークン付き）が残るため、
			// 元リクエストへ差し戻されトークンが残っていないことも検証する
			if resp.Request != nil {
				if got := resp.Request.URL.Query().Get("i"); got != "" {
					t.Errorf("Response.Requestのトークンが差し戻っていない: i = %q", got)
				}
			}
		})
	}
}
