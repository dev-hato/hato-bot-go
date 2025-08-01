package misskey_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"hato-bot-go/lib"
	libHttp "hato-bot-go/lib/http"
	"hato-bot-go/lib/misskey"
)

func TestAddReaction(t *testing.T) {
	tests := []struct {
		name        string
		noteID      string
		reaction    string
		statusCode  int
		expectError error
	}{
		{
			name:        "正常なリアクション追加",
			noteID:      "note123",
			reaction:    "👍",
			statusCode:  http.StatusNoContent,
			expectError: nil,
		},
		{
			name:        "APIエラー応答",
			noteID:      "note456",
			reaction:    "❤️",
			statusCode:  http.StatusBadRequest,
			expectError: libHttp.ErrHTTPRequestError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runSimpleBotTest(t, tt.statusCode, func(bot *misskey.Bot) error {
				return bot.AddReaction(context.Background(), tt.noteID, tt.reaction)
			}, tt.expectError, "AddReaction()")
		})
	}
}

func TestCreateNote(t *testing.T) {
	tests := []struct {
		name         string
		req          *misskey.CreateNoteRequest
		statusCode   int
		responseBody string
		expectError  error
	}{
		{
			name:         "nilリクエスト",
			req:          nil,
			statusCode:   http.StatusOK,
			responseBody: `{"createdNote":{"id":"created123"}}`,
			expectError:  lib.ErrParamsNil,
		},
		{
			name: "nil OriginalNote",
			req: &misskey.CreateNoteRequest{
				Text:         "test",
				OriginalNote: nil,
			},
			statusCode:   http.StatusOK,
			responseBody: `{"createdNote":{"id":"created123"}}`,
			expectError:  lib.ErrParamsNil,
		},
		{
			name: "有効なリクエスト",
			req: &misskey.CreateNoteRequest{
				Text: "test note",
				OriginalNote: &misskey.Note{
					ID:         "original123",
					Visibility: "home",
				},
			},
			statusCode:   http.StatusOK,
			responseBody: `{"createdNote":{"id":"created123"}}`,
			expectError:  nil,
		},
		{
			name: "APIエラー応答",
			req: &misskey.CreateNoteRequest{
				Text: "test note",
				OriginalNote: &misskey.Note{
					ID:         "original123",
					Visibility: "home",
				},
			},
			statusCode:   http.StatusBadRequest,
			responseBody: `{"error":"bad request"}`,
			expectError:  libHttp.ErrHTTPRequestError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runBotTest(t, tt.statusCode, tt.responseBody, func(bot *misskey.Bot) error {
				return bot.CreateNote(context.Background(), tt.req)
			}, tt.expectError, "CreateNote()")
		})
	}
}

func TestUploadFile(t *testing.T) {
	tests := []struct {
		name         string
		fileName     string
		readerData   string
		statusCode   int
		responseBody string
		expectError  error
	}{
		{
			name:         "成功したファイルアップロード",
			fileName:     "test.txt",
			readerData:   "test file content",
			statusCode:   http.StatusOK,
			responseBody: `{"id":"file123","name":"test.txt","url":"https://example.com/file123"}`,
			expectError:  nil,
		},
		{
			name:         "APIエラー応答",
			fileName:     "test.txt",
			readerData:   "test content",
			statusCode:   http.StatusBadRequest,
			responseBody: `{"error":"bad request"}`,
			expectError:  libHttp.ErrHTTPRequestError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Helper()
			mockClient := libHttp.NewMockHTTPClient(tt.statusCode, tt.responseBody)
			bot := misskey.NewBotWithClient(&misskey.BotSetting{
				Domain: "example.com",
				Token:  "token",
				Client: mockClient,
			})

			reader := strings.NewReader(tt.readerData)
			if _, err := bot.UploadFile(context.Background(), reader, tt.fileName); !errors.Is(err, tt.expectError) {
				t.Errorf("UploadFile() error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}

func TestProcessAmeshCommand(t *testing.T) {
	tests := []struct {
		name        string
		req         *misskey.ProcessAmeshCommandRequest
		expectError error
	}{
		{
			name: "nilノート",
			req: &misskey.ProcessAmeshCommandRequest{
				Note:          nil,
				Place:         "東京",
				YahooAPIToken: "YahooAPIToken",
			},
			expectError: lib.ErrParamsNil,
		},
		{
			name: "Yahoo APIトークンが設定されていない",
			req: &misskey.ProcessAmeshCommandRequest{
				Note: &misskey.Note{
					ID:         "note123",
					Visibility: "home",
				},
				Place: "東京",
			},
			expectError: misskey.ErrParamsEmptyString, // Yahoo APIトークンが設定されていないためエラーが発生する
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runSimpleBotTest(t, http.StatusNoContent, func(bot *misskey.Bot) error {
				return bot.ProcessAmeshCommand(context.Background(), tt.req)
			}, tt.expectError, "ProcessAmeshCommand()")
		})
	}
}
