package http

import (
	"net/http"
)

// Client はHTTPリクエストを行うインターフェース
type Client interface {
	Get(url string) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
}

// DefaultHTTPClient はデフォルトのHTTPクライアント
var DefaultHTTPClient Client = &http.Client{}
