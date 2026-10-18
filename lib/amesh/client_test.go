package amesh

import (
	"testing"
	"time"
)

// TestDefaultClientHasTimeout 既定のHTTPクライアントにタイムアウトが設定されていることをテストする
// 外部APIが応答しない場合にリクエストが終わらなくならないようにするため
func TestDefaultClientHasTimeout(t *testing.T) {
	t.Parallel()

	want := 10 * time.Second

	if defaultClient.Timeout != want {
		t.Errorf("defaultClient.Timeout = %v, want %v", defaultClient.Timeout, want)
	}
}
