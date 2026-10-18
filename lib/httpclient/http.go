package httpclient

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/cockroachdb/errors"

	"hato-bot-go/lib"
)

var ErrHTTPRequestError = errors.New("A http request returned error status")

// ExecuteHTTPRequest HTTPリクエストを実行し、共通のエラーハンドリングを行う
func ExecuteHTTPRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "hato-bot-go/"+lib.Version)

	resp, err := client.Do(req) //nolint:gosec //G704
	if err != nil {
		return nil, errors.Wrap(redactURLError(err), "Failed to Do")
	}

	// レスポンスステータスを確認
	if !slices.Contains([]int{http.StatusOK, http.StatusAccepted, http.StatusNoContent}, resp.StatusCode) {
		if err := resp.Body.Close(); err != nil {
			return nil, errors.Wrap(err, "Failed to Close")
		}

		return nil, errors.Wrapf(ErrHTTPRequestError, "ステータス %d", resp.StatusCode)
	}

	return resp, nil
}

// redactURLError 通信エラーのメッセージからURLのクエリ文字列を取り除く
// クエリ文字列にはAPIキーや署名付きURLの署名が含まれることがあり、エラーがログに出力されると漏洩するため
func redactURLError(err error) error {
	var urlErr *url.Error

	if !errors.As(err, &urlErr) {
		return err
	}

	parsed, parseErr := url.Parse(urlErr.URL)
	if parseErr != nil {
		urlErr.URL = "[REDACTED]"
		return err
	}

	if parsed.RawQuery != "" {
		parsed.RawQuery = "[REDACTED]"
	}

	parsed.User = nil
	parsed.Fragment = ""
	urlErr.URL = parsed.String()
	return err
}
