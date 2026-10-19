package httpclient

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
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

// redactURLError 通信エラーのメッセージからURLのパスやクエリ文字列を取り除く
// パスやクエリ文字列にはAPIキーや署名付きURLの署名が含まれることがあり、エラーがログに出力されると漏洩するため
func redactURLError(err error) error {
	var urlErr *url.Error

	if !errors.As(err, &urlErr) {
		return err
	}

	urlErr.URL = redactURL(urlErr.URL)

	// 転送先のLocationヘッダーを解析できない場合などは、原因エラーのメッセージにもURLが埋め込まれる
	if urlErr.Err != nil {
		urlErr.Err = redactCauseError(urlErr.Err)
	}

	return err
}

// redactURL URLからスキームとホスト以外（ユーザー情報・パス・クエリ文字列・フラグメント）を取り除く
func redactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "[REDACTED]"
	}

	if parsed.Opaque != "" {
		parsed.Opaque = "[REDACTED]"
	}

	if parsed.Path != "" && parsed.Path != "/" {
		// RawPathも設定し、角括弧がエスケープされずにそのまま表示されるようにする
		parsed.Path = "/[REDACTED]"
		parsed.RawPath = "/[REDACTED]"
	}

	if parsed.RawQuery != "" {
		parsed.RawQuery = "[REDACTED]"
	}

	parsed.User = nil
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return parsed.String()
}

var (
	// quotedInTextPattern 文字列に埋め込まれた引用符付きの値
	// net/httpやnet/urlはURLを引用符で囲んでエラーメッセージに埋め込むため、相対URLも含めて値ごと伏せる
	quotedInTextPattern = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

	// urlInTextPattern 文字列に埋め込まれた引用符なしの絶対URL
	urlInTextPattern = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s"]+`)

	// queryInTextPattern 文字列に埋め込まれたURLのクエリ文字列とフラグメント
	queryInTextPattern = regexp.MustCompile(`[?#][^\s"]*`)
)

// redactedCauseError 秘密情報を取り除いた原因エラー
type redactedCauseError struct {
	msg   string // 秘密情報を取り除いたメッセージ
	cause error  // 呼び出し元が判定に使うcontextのエラー（それ以外の原因エラーは秘密情報を含みうるため保持しない）
}

func (e *redactedCauseError) Error() string {
	return e.msg
}

func (e *redactedCauseError) Unwrap() error {
	return e.cause
}

// redactCauseError 原因エラーのメッセージに埋め込まれたURLから秘密情報を取り除く
// 元のエラーを保持すると%+vなどで元のメッセージが表示されるため、メッセージを書き換えた別のエラーに置き換える
// 取り除くものがない場合は、呼び出し元がエラーの種類を判定できるよう元のエラーをそのまま返す
func redactCauseError(err error) error {
	msg := quotedInTextPattern.ReplaceAllString(err.Error(), `"[REDACTED]"`)
	msg = urlInTextPattern.ReplaceAllStringFunc(msg, redactURL)
	msg = queryInTextPattern.ReplaceAllString(msg, "?[REDACTED]")

	if msg == err.Error() {
		return err
	}

	var cause error

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		cause = context.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		cause = context.Canceled
	}

	return &redactedCauseError{msg: msg, cause: cause}
}
