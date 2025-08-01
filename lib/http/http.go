package http

import (
	"io"
	"net/http"

	"github.com/cockroachdb/errors"
)

var ErrGeocodingAPIError = errors.New("geocoding API returned error status")

// ExecuteHTTPRequest HTTPリクエストを実行し、共通のエラーハンドリングを行う
func ExecuteHTTPRequest(client *http.Client, req *http.Request) (resq *http.Response, err error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to Do")
	}
	defer func(Body io.ReadCloser) {
		if closeErr := Body.Close(); closeErr != nil {
			err = closeErr
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		if err := resp.Body.Close(); err != nil {
			return nil, errors.Wrap(err, "Failed to Close")
		}

		return nil, errors.Wrapf(ErrGeocodingAPIError, "ステータス %d", resp.StatusCode)
	}

	return resp, nil
}
