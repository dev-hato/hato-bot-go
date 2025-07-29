package http

import (
	"io"
	"net/http"
	"strings"
)

// MockHTTPClient はテスト用のHTTPクライアントのモック
type MockHTTPClient struct {
	GetFunc func(url string) (*http.Response, error)
	DoFunc  func(req *http.Request) (*http.Response, error)
}

// Get HTTPのGETリクエストを実行する
func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
	if m.GetFunc != nil {
		return m.GetFunc(url)
	}
	return nil, nil
}

// Do HTTPリクエストを実行する
func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return nil, nil
}

// NewMockHTTPClient 指定されたステータスコードとレスポンスボディでモックHTTPクライアントを作成する
func NewMockHTTPClient(statusCode int, responseBody string) *MockHTTPClient {
	return &MockHTTPClient{
		DoFunc: func(_ *http.Request) (*http.Response, error) {
			return createMockResponse(statusCode, responseBody), nil
		},
		GetFunc: func(_ string) (*http.Response, error) {
			return createMockResponse(statusCode, responseBody), nil
		},
	}
}

// createMockResponse 指定されたステータスコードとレスポンスボディでHTTPレスポンスを作成する
func createMockResponse(statusCode int, responseBody string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
		Header:     make(http.Header),
	}
}
