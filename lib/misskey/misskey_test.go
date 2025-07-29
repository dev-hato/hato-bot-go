package misskey_test

import (
	"hato-bot-go/lib/misskey"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseAmeshCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected misskey.ParseResult
	}{
		{
			name:     "シンプルなameshコマンド",
			input:    "amesh 東京",
			expected: misskey.ParseResult{Place: "東京", IsAmesh: true},
		},
		{
			name:     "場所無しのameshコマンドは東京がデフォルト",
			input:    "amesh",
			expected: misskey.ParseResult{Place: "東京", IsAmesh: true},
		},
		{
			name:     "メンション付きameshコマンド",
			input:    "@bot amesh 大阪",
			expected: misskey.ParseResult{Place: "大阪", IsAmesh: true},
		},
		{
			name:     "複数メンション付きameshコマンド",
			input:    "@bot @user amesh 名古屋",
			expected: misskey.ParseResult{Place: "名古屋", IsAmesh: true},
		},
		{
			name:     "余分な空白付きameshコマンド",
			input:    "  amesh   福岡  ",
			expected: misskey.ParseResult{Place: "福岡", IsAmesh: true},
		},
		{
			name:     "複数単語の場所名を持つameshコマンド",
			input:    "amesh 新宿 駅",
			expected: misskey.ParseResult{Place: "新宿 駅", IsAmesh: true},
		},
		{
			name:     "ameshコマンドではないテキスト",
			input:    "hello world",
			expected: misskey.ParseResult{Place: "", IsAmesh: false},
		},
		{
			name:     "部分的なameshコマンド",
			input:    "ameshi",
			expected: misskey.ParseResult{Place: "", IsAmesh: false},
		},
		{
			name:     "ameshが単語の一部に含まれる場合",
			input:    "gameshow",
			expected: misskey.ParseResult{Place: "", IsAmesh: false},
		},
		{
			name:     "空の入力",
			input:    "",
			expected: misskey.ParseResult{Place: "", IsAmesh: false},
		},
		{
			name:     "メンションのみ",
			input:    "@bot @user",
			expected: misskey.ParseResult{Place: "", IsAmesh: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := misskey.ParseAmeshCommand(tt.input)
			if diff := cmp.Diff(result, tt.expected); diff != "" {
				t.Errorf("ParseAmeshCommand(%q) diff: %s", tt.input, diff)
			}
		})
	}
}

// MockHTTPClient HTTPクライアントのモック
type MockHTTPClient struct {
	DoFunc  func(req *http.Request) (*http.Response, error)
	GetFunc func(url string) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return nil, nil
}

func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
	if m.GetFunc != nil {
		return m.GetFunc(url)
	}
	return nil, nil
}

// createMockHTTPClient 指定されたステータスコードとレスポンスボディでモックHTTPクライアントを作成する
func createMockHTTPClient(statusCode int, responseBody string) *MockHTTPClient {
	return &MockHTTPClient{
		DoFunc: func(_ *http.Request) (*http.Response, error) {
			resp := &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(strings.NewReader(responseBody)),
			}
			return resp, nil
		},
	}
}

// runBotTest HTTPクライアントのモック付きでボットテストを実行する共通ヘルパー
func runBotTest(t *testing.T, statusCode int, responseBody string, testFunc func(*misskey.Bot) error, expectError bool, testName string) {
	t.Helper()
	mockClient := createMockHTTPClient(statusCode, responseBody)
	bot := misskey.NewBotWithClient("example.com", "token", mockClient)

	err := testFunc(bot)
	if (err != nil) != expectError {
		t.Errorf("%s error = %v, expectError %v", testName, err, expectError)
	}
}

// runSimpleBotTest 空のレスポンスボディでボットテストを実行する共通ヘルパー
func runSimpleBotTest(t *testing.T, statusCode int, testFunc func(*misskey.Bot) error, expectError bool, testName string) {
	runBotTest(t, statusCode, "", testFunc, expectError, testName)
}

// runBotTestWithResult 戻り値を無視してエラーのみチェックするボットテスト実行ヘルパー
func runBotTestWithResult(t *testing.T, statusCode int, responseBody string, testFunc func(*misskey.Bot) (interface{}, error), expectError bool, testName string) {
	t.Helper()
	mockClient := createMockHTTPClient(statusCode, responseBody)
	bot := misskey.NewBotWithClient("example.com", "token", mockClient)

	_, err := testFunc(bot)
	if (err != nil) != expectError {
		t.Errorf("%s error = %v, expectError %v", testName, err, expectError)
	}
}

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
			runSimpleBotTest(t, tt.statusCode, func(bot *misskey.Bot) error {
				return bot.AddReaction(tt.noteID, tt.reaction)
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
			runBotTest(t, tt.statusCode, tt.responseBody, func(bot *misskey.Bot) error {
				return bot.CreateNote(tt.req)
			}, tt.expectError, "CreateNote()")
		})
	}
}

func TestUploadFile(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		statusCode   int
		responseBody string
		expectError  bool
	}{
		{
			name:         "存在しないファイル",
			filePath:     "/nonexistent/file.txt",
			statusCode:   200,
			responseBody: `{"id":"file123","name":"test.txt","url":"https://example.com/file123"}`,
			expectError:  true, // ファイルが存在しないためエラーになる
		},
		{
			name:         "APIエラー応答",
			filePath:     "/tmp/test.txt", // 実際には呼ばれない
			statusCode:   400,
			responseBody: `{"error":"bad request"}`,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runBotTestWithResult(t, tt.statusCode, tt.responseBody, func(bot *misskey.Bot) (interface{}, error) {
				return bot.UploadFile(tt.filePath)
			}, tt.expectError, "UploadFile()")
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
			runSimpleBotTest(t, 204, func(bot *misskey.Bot) error {
				return bot.ProcessAmeshCommand(tt.note, tt.place)
			}, tt.expectError, "ProcessAmeshCommand()")
		})
	}
}
