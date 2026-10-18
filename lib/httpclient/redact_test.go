package httpclient

import (
	"net/url"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
)

// TestRedactURLError 通信エラーのURLから秘密情報を取り除くことをテストする
func TestRedactURLError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rawURL     string
		wantURL    string
		notContain []string
	}{
		{
			name:       "ユーザー情報・クエリ文字列・フラグメントを取り除く",
			rawURL:     "https://user:dummy-password@example.com/path?appid=dummy-app-id#dummy-fragment",
			wantURL:    "https://example.com/path?[REDACTED]",
			notContain: []string{"user", "dummy-password", "dummy-app-id", "dummy-fragment"},
		},
		{
			name:       "クエリ文字列がなければそのまま",
			rawURL:     "https://example.com/path",
			wantURL:    "https://example.com/path",
			notContain: nil,
		},
		{
			name:       "解析できないURLは全体を伏せる",
			rawURL:     "https://example.com/%zz?appid=dummy-app-id",
			wantURL:    "[REDACTED]",
			notContain: []string{"dummy-app-id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := redactURLError(errors.Wrap(
				&url.Error{Op: "Get", URL: tt.rawURL, Err: errors.New("connection refused")},
				"wrapped",
			))
			var urlErr *url.Error

			// ラップされた*url.Errorも書き換えられることを確認する
			if !errors.As(err, &urlErr) {
				t.Fatalf("redactURLError() = %v, want *url.Error", err)
			}
			if urlErr.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", urlErr.URL, tt.wantURL)
			}

			for _, s := range tt.notContain {
				if strings.Contains(err.Error(), s) {
					t.Errorf("エラーメッセージに %q が含まれている: %s", s, err.Error())
				}
			}
		})
	}
}

// TestRedactURLErrorKeepsOtherErrors *url.Error以外のエラーはそのまま返すことをテストする
func TestRedactURLErrorKeepsOtherErrors(t *testing.T) {
	t.Parallel()

	original := errors.New("other error")

	if got := redactURLError(original); !errors.Is(got, original) { // 同じエラーをそのまま返すことを確認するため
		t.Errorf("redactURLError() = %v, want %v", got, original)
	}
}
