package mixi2

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"go.uber.org/mock/gomock"
)

// TestUploadMediaRejectsInsecureURL HTTPS以外のアップロード先にはアクセストークンを送らないことをテストする
func TestUploadMediaRejectsInsecureURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		uploadURL string
	}{
		{name: "HTTP", uploadURL: "http://example.com/upload"},
		{name: "スキームなし", uploadURL: "example.com/upload"},
		{name: "不正なURL", uploadURL: "://example.com"},
		{name: "ホストなし", uploadURL: "https:///upload"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// アクセストークンを取得しないことを確認するため、何も呼ばれない想定のモックを使う
			handler := &Handler{Authenticator: NewMockAuthenticator(gomock.NewController(t))}

			err := handler.uploadMedia(t.Context(), tt.uploadURL, &bytes.Buffer{})
			if !errors.Is(err, ErrInsecureUploadURL) {
				t.Errorf("uploadMedia() error = %v, want %v", err, ErrInsecureUploadURL)
			}
		})
	}
}

// uploadRecord アップロードリクエストの記録
type uploadRecord struct {
	method        string
	url           string
	authorization string
	body          string
}

// recordingUploadRoundTrip アップロードリクエストを記録して200を返すモック
type recordingUploadRoundTrip struct {
	record *uploadRecord
}

func (r recordingUploadRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to io.ReadAll")
	}

	*r.record = uploadRecord{
		method:        req.Method,
		url:           req.URL.String(),
		authorization: req.Header.Get("Authorization"),
		body:          string(body),
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

// TestUploadMediaSendsToHTTPSURL HTTPSのアップロード先にはアクセストークン付きで送信することをテストする
func TestUploadMediaSendsToHTTPSURL(t *testing.T) {
	t.Parallel()

	const (
		uploadURL   = "https://upload.example.com/media?signature=dummy"
		accessToken = "dummy-access-token"
		mediaData   = "dummy-media-data"
	)

	mockAuth := NewMockAuthenticator(gomock.NewController(t))
	mockAuth.EXPECT().
		GetAccessToken(gomock.Any()).
		Return(accessToken, nil)

	record := &uploadRecord{}
	handler := &Handler{
		Authenticator: mockAuth,
		HTTPClient:    &http.Client{Transport: recordingUploadRoundTrip{record: record}},
	}

	if err := handler.uploadMedia(t.Context(), uploadURL, bytes.NewBufferString(mediaData)); err != nil {
		t.Fatalf("uploadMedia() error = %v", err)
	}

	want := uploadRecord{
		method:        http.MethodPost,
		url:           uploadURL,
		authorization: "Bearer " + accessToken,
		body:          mediaData,
	}

	if *record != want {
		t.Errorf("upload request = %+v, want %+v", *record, want)
	}
}
