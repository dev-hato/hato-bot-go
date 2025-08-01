package misskey_test

import (
	"context"
	"strings"
	"testing"

	"hato-bot-go/lib/http"
	"hato-bot-go/lib/misskey"
)

func TestAddReaction(t *testing.T) {
	tests := []struct {
		name        string
		noteID      string
		reaction    string
		statusCode  int
		expectError bool
	}{
		{
			name:        "正常なリアクション追加",
			noteID:      "note123",
			reaction:    "👍",
			statusCode:  204,
			expectError: false,
		},
		{
			name:        "APIエラー応答",
			noteID:      "note456",
			reaction:    "❤️",
			statusCode:  400,
			expectError: true,
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
		expectError  bool
	}{
		{
			name:         "nilリクエスト",
			req:          nil,
			statusCode:   200,
			responseBody: `{"createdNote":{"id":"created123"}}`,
			expectError:  true,
		},
		{
			name: "nil OriginalNote",
			req: &misskey.CreateNoteRequest{
				Text:         "test",
				OriginalNote: nil,
			},
			statusCode:   200,
			responseBody: `{"createdNote":{"id":"created123"}}`,
			expectError:  true,
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
			statusCode:   200,
			responseBody: `{"createdNote":{"id":"created123"}}`,
			expectError:  false,
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
			statusCode:   400,
			responseBody: `{"error":"bad request"}`,
			expectError:  true,
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

func TestUploadFileFromReader(t *testing.T) {
	tests := []struct {
		name         string
		fileName     string
		readerData   string
		statusCode   int
		responseBody string
		expectError  bool
	}{
		{
			name:         "成功したファイルアップロード",
			fileName:     "test.txt",
			readerData:   "test file content",
			statusCode:   200,
			responseBody: `{"id":"file123","name":"test.txt","url":"https://example.com/file123"}`,
			expectError:  false,
		},
		{
			name:         "APIエラー応答",
			fileName:     "test.txt",
			readerData:   "test content",
			statusCode:   400,
			responseBody: `{"error":"bad request"}`,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Helper()
			mockClient := http.NewMockHTTPClient(tt.statusCode, tt.responseBody)
			bot := misskey.NewBotWithClient(&misskey.BotSetting{
				Domain: "example.com",
				Token:  "token",
				Client: mockClient,
			})

			reader := strings.NewReader(tt.readerData)
			if _, err := bot.UploadFileFromReader(context.Background(), reader, tt.fileName); (err != nil) != tt.expectError {
				t.Errorf("UploadFileFromReader() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestProcessAmeshCommand(t *testing.T) {
	tests := []struct {
		name        string
		note        *misskey.Note
		place       string
		expectError bool
	}{
		{
			name:        "nilノート",
			note:        nil,
			place:       "東京",
			expectError: true,
		},
		{
			name: "Yahoo APIトークンが設定されていない",
			note: &misskey.Note{
				ID:         "note123",
				Visibility: "home",
			},
			place:       "東京",
			expectError: true, // Yahoo APIトークンが設定されていないためエラーが発生する
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runSimpleBotTest(t, 204, func(bot *misskey.Bot) error {
				return bot.ProcessAmeshCommand(context.Background(), tt.note, tt.place)
			}, tt.expectError, "ProcessAmeshCommand()")
		})
	}
}
