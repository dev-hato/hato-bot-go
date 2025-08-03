package http

import (
	"io"
	"net/http"
	"strings"
)

type roundTrip struct {
	statusCode   int
	responseBody string
}

func (f roundTrip) RoundTrip(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: f.statusCode,
		Body:       io.NopCloser(strings.NewReader(f.responseBody)),
		Header:     make(http.Header),
	}, nil
}

// NewMockHTTPClient 指定されたステータスコードとレスポンスボディでモックHTTPクライアントを作成する
func NewMockHTTPClient(statusCode int, responseBody string) *http.Client {
	return &http.Client{
		Transport: roundTrip{
			statusCode:   statusCode,
			responseBody: responseBody,
		},
	}
}
