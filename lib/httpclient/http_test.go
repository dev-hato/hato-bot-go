package httpclient_test

import (
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

// TestExecuteHTTPRequestRedactsQuery 通信エラーのメッセージにクエリ文字列（APIキーなど）を含めないことをテストする
func TestExecuteHTTPRequestRedactsQuery(t *testing.T) {
	t.Parallel()

	const appID = "dummy-app-id-for-test"

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"https://example.com/geocode?appid="+appID+"&query=tokyo",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = httpclient.ExecuteHTTPRequest(&http.Client{Transport: errorRoundTrip{}}, req); err == nil {
		t.Fatal("ExecuteHTTPRequest() error = nil, want error")
	}

	for _, msg := range []string{err.Error(), fmt.Sprintf("%+v", err)} {
		if strings.Contains(msg, appID) {
			t.Errorf("エラーメッセージにAPIキーが含まれている: %s", msg)
		}
	}
}
