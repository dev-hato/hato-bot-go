package http

import (
	"net/http"
	"slices"

	"github.com/cockroachdb/errors"
)

var ErrHTTPRequestError = errors.New("A http request returned error status")

// ExecuteHTTPRequest HTTPリクエストを実行し、共通のエラーハンドリングを行う
func ExecuteHTTPRequest(client *http.Client, req *http.Request) (resq *http.Response, err error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to Do")
	}

	if !slices.Contains([]int{http.StatusOK, http.StatusNoContent}, resp.StatusCode) {
		if err := resp.Body.Close(); err != nil {
			return nil, errors.Wrap(err, "Failed to Close")
		}

		return nil, errors.Wrapf(ErrHTTPRequestError, "ステータス %d", resp.StatusCode)
	}

	return resp, nil
}
