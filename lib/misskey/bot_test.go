package misskey_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"hato-bot-go/lib"
	"hato-bot-go/lib/httpclient"
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
			expectError: httpclient.ErrHTTPRequestError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runSimpleBotTest(t, &runSimpleBotTestParams{
				StatusCode: tt.statusCode,
				TestFunc: func(bot *misskey.Bot) error {
					return bot.AddReaction(t.Context(), tt.noteID, tt.reaction)
				},
				ExpectError: tt.expectError,
				TestName:    "AddReaction()",
			})
		})
	}
}

func TestCreateNote(t *testing.T) {
	tests := []struct {
		name         string
		params       *misskey.CreateNoteParams
		statusCode   int
		responseBody string
		expectError  error
	}{
		{
			name:         "nilリクエスト",
			params:       nil,
			statusCode:   http.StatusOK,
			responseBody: `{"createdNote":{"id":"created123"}}`,
			expectError:  lib.ErrParamsNil,
		},
		{
			name: "nil OriginalNote",
			params: &misskey.CreateNoteParams{
				Text:         "test",
				OriginalNote: nil,
			},
			statusCode:   http.StatusOK,
			responseBody: `{"createdNote":{"id":"created123"}}`,
			expectError:  lib.ErrParamsNil,
		},
		{
			name: "有効なリクエスト",
			params: &misskey.CreateNoteParams{
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
		// jscpd:ignore-start
		{
			name: "APIエラー応答",
			params: &misskey.CreateNoteParams{
				Text: "test note",
				OriginalNote: &misskey.Note{
					ID:         "original123",
					Visibility: "home",
				},
			},
			statusCode:   http.StatusBadRequest,
			responseBody: `{"error":"bad request"}`,
			expectError:  httpclient.ErrHTTPRequestError,
		},
		// jscpd:ignore-end
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runBotTest(t, &runBotTestParams{
				StatusCode:   tt.statusCode,
				ResponseBody: tt.responseBody,
				TestFunc: func(bot *misskey.Bot) error {
					return bot.CreateNote(t.Context(), tt.params)
				},
				ExpectError: tt.expectError,
				TestName:    "CreateNote()",
			})
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
		// jscpd:ignore-start
		{
			name:         "APIエラー応答",
			fileName:     "test.txt",
			readerData:   "test content",
			statusCode:   http.StatusBadRequest,
			responseBody: `{"error":"bad request"}`,
			expectError:  httpclient.ErrHTTPRequestError,
		},
		// jscpd:ignore-end
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Helper()
			mockClient := httpclient.NewMockHTTPClient(tt.statusCode, tt.responseBody)
			bot := misskey.NewBotWithClient(&misskey.BotSetting{
				Domain: "example.com",
				Token:  "token",
				Client: mockClient,
			})

			reader := strings.NewReader(tt.readerData)
			if _, err := bot.UploadFile(t.Context(), reader, tt.fileName); !errors.Is(err, tt.expectError) {
				t.Errorf("UploadFile() error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}

func TestProcessAmeshCommand(t *testing.T) {
	tests := []struct {
		name        string
		params      *misskey.ProcessAmeshCommandParams
		expectError error
	}{
		{
			name:        "nilリクエスト",
			params:      nil,
			expectError: lib.ErrParamsNil,
		},
		{
			name: "nilノート",
			params: &misskey.ProcessAmeshCommandParams{
				Note:          nil,
				Place:         "東京",
				YahooAPIToken: "YahooAPIToken",
			},
			expectError: lib.ErrParamsNil,
		},
		{
			name: "Yahoo APIトークンが設定されていない",
			params: &misskey.ProcessAmeshCommandParams{
				Note: &misskey.Note{
					ID:         "note123",
					Visibility: "home",
				},
				Place: "東京",
			},
			expectError: lib.ErrParamsEmptyString, // Yahoo APIトークンが設定されていないためエラーが発生する
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runSimpleBotTest(t, &runSimpleBotTestParams{
				StatusCode: http.StatusNoContent,
				TestFunc: func(bot *misskey.Bot) error {
					return bot.ProcessAmeshCommand(t.Context(), tt.params)
				},
				ExpectError: tt.expectError,
				TestName:    "ProcessAmeshCommand()",
			})
		})
	}
}
