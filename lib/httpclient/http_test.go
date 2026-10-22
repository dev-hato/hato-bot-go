package httpclient_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"hato-bot-go/lib/httpclient"
)

// errorRoundTrip 常に通信エラーを返すモック
type errorRoundTrip struct{}

func (errorRoundTrip) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

// assertErrorRedacted リクエストがエラーになり、そのメッセージに秘密情報が含まれないことを確認する
func assertErrorRedacted(t *testing.T, client *http.Client, rawURL, secret string) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = httpclient.ExecuteHTTPRequest(client, req); err == nil {
		t.Fatal("ExecuteHTTPRequest() error = nil, want error")
	}

	for _, msg := range []string{err.Error(), fmt.Sprintf("%+v", err)} {
		if strings.Contains(msg, secret) {
			t.Errorf("エラーメッセージに秘密情報が含まれている: %s", msg)
		}
	}
}

// TestExecuteHTTPRequestRedactsQuery 通信エラーのメッセージにクエリ文字列（APIキーなど）を含めないことをテストする
func TestExecuteHTTPRequestRedactsQuery(t *testing.T) {
	t.Parallel()

	const appID = "dummy-app-id-for-test"

	assertErrorRedacted(
		t,
		&http.Client{Transport: errorRoundTrip{}},
		"https://example.com/geocode?appid="+appID+"&query=tokyo",
		appID,
	)
}

// redirectRoundTrip 指定したLocationヘッダーで302を返すモック
type redirectRoundTrip struct {
	location string
}

func (r redirectRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	header := make(http.Header)
	header.Set("Location", r.location)

	return &http.Response{
		StatusCode: http.StatusFound,
		Body:       http.NoBody,
		Header:     header,
		Request:    req,
	}, nil
}

// TestExecuteHTTPRequestRedactsPath 通信エラーのメッセージにURLのパス（署名を含みうる）を含めないことをテストする
func TestExecuteHTTPRequestRedactsPath(t *testing.T) {
	t.Parallel()

	const token = "dummy-path-token-for-test"

	assertErrorRedacted(t, &http.Client{Transport: errorRoundTrip{}}, "https://upload.example.com/upload/"+token, token)
}

// TestExecuteHTTPRequestRedactsCauseError 原因エラーに埋め込まれたURLのクエリ文字列もエラーメッセージに含めないことをテストする
// 不正なエスケープを含むLocationヘッダーは、ヘッダー全体が原因エラーのメッセージに埋め込まれる
func TestExecuteHTTPRequestRedactsCauseError(t *testing.T) {
	t.Parallel()

	const signature = "dummy-signature-for-test"

	tests := []struct {
		name     string
		location string
	}{
		{name: "パスに不正なエスケープ", location: "https://upload.example.com/media%zz?signature=" + signature},
		{name: "相対URLのパスに不正なエスケープ", location: "/media%zz?signature=" + signature},
		{
			name:     "フラグメントに不正なエスケープ",
			location: "https://upload.example.com/media?signature=" + signature + "#%zz",
		},
		{name: "不正なポート", location: "https://upload.example.com:bad/media?signature=" + signature},
		{name: "ユーザー情報", location: "https://user:" + signature + "@upload.example.com:bad/media"},
		{name: "パスに署名", location: "https://upload.example.com/" + signature + "/%zz"},
		{name: "相対URLのパスに署名", location: "/" + signature + "/%zz"},
		{name: "スラッシュで始まらない相対URLのパスに署名", location: signature + "/%zz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertErrorRedacted(
				t,
				&http.Client{Transport: redirectRoundTrip{location: tt.location}},
				"https://upload.example.com/media",
				signature,
			)
		})
	}
}

// deadlineRoundTrip 常にタイムアウトのエラーを返すモック
type deadlineRoundTrip struct{}

func (deadlineRoundTrip) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, context.DeadlineExceeded
}

// TestExecuteHTTPRequestKeepsDeadlineExceeded 秘密情報を取り除いた後もタイムアウトであることを判定できることをテストする
func TestExecuteHTTPRequestKeepsDeadlineExceeded(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"https://example.com/?appid=dummy-app-id",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = httpclient.ExecuteHTTPRequest(&http.Client{Transport: deadlineRoundTrip{}}, req)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("ExecuteHTTPRequest() error = %v, want %v", err, context.DeadlineExceeded)
	}
}
