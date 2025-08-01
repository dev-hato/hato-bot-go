package http

import (
	"net/http"

	"github.com/cockroachdb/errors"
)

// ExecuteHTTPRequest HTTPリクエストを実行し、共通のエラーハンドリングを行う
func ExecuteHTTPRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to Do")
	}
	return resp, nil
}
