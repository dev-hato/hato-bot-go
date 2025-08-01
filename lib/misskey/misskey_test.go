package misskey_test

import (
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/go-cmp/cmp"

	"hato-bot-go/lib/http"
	"hato-bot-go/lib/misskey"
)

type runSimpleBotTestParams struct {
	StatusCode  int
	TestFunc    func(*misskey.Bot) error
	ExpectError error
	TestName    string
}

type runBotTestParams struct {
	StatusCode   int
	ResponseBody string
	TestFunc     func(*misskey.Bot) error
	ExpectError  error
	TestName     string
}

func TestParseAmeshCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected misskey.ParseAmeshCommandResult
	}{
		{
			name:     "シンプルなameshコマンド",
			input:    "amesh 東京",
			expected: misskey.ParseAmeshCommandResult{Place: "東京", IsAmesh: true},
		},
		{
			name:     "場所無しのameshコマンドは東京がデフォルト",
			input:    "amesh",
			expected: misskey.ParseAmeshCommandResult{Place: "東京", IsAmesh: true},
		},
		{
			name:     "メンション付きameshコマンド",
			input:    "@bot amesh 大阪",
			expected: misskey.ParseAmeshCommandResult{Place: "大阪", IsAmesh: true},
		},
		{
			name:     "複数メンション付きameshコマンド",
			input:    "@bot @user amesh 名古屋",
			expected: misskey.ParseAmeshCommandResult{Place: "名古屋", IsAmesh: true},
		},
		{
			name:     "余分な空白付きameshコマンド",
			input:    "  amesh   福岡  ",
			expected: misskey.ParseAmeshCommandResult{Place: "福岡", IsAmesh: true},
		},
		{
			name:     "複数単語の場所名を持つameshコマンド",
			input:    "amesh 新宿 駅",
			expected: misskey.ParseAmeshCommandResult{Place: "新宿 駅", IsAmesh: true},
		},
		{
			name:     "ameshコマンドではないテキスト",
			input:    "hello world",
			expected: misskey.ParseAmeshCommandResult{Place: "", IsAmesh: false},
		},
		{
			name:     "部分的なameshコマンド",
			input:    "ameshi",
			expected: misskey.ParseAmeshCommandResult{Place: "", IsAmesh: false},
		},
		{
			name:     "ameshが単語の一部に含まれる場合",
			input:    "gameshow",
			expected: misskey.ParseAmeshCommandResult{Place: "", IsAmesh: false},
		},
		{
			name:     "空の入力",
			input:    "",
			expected: misskey.ParseAmeshCommandResult{Place: "", IsAmesh: false},
		},
		{
			name:     "メンションのみ",
			input:    "@bot @user",
			expected: misskey.ParseAmeshCommandResult{Place: "", IsAmesh: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := misskey.ParseAmeshCommand(tt.input)
			if diff := cmp.Diff(result, tt.expected); diff != "" {
				t.Errorf("ParseAmeshCommand(%q) diff: %s", tt.input, diff)
			}
		})
	}
}

// runSimpleBotTest 空のレスポンスボディでボットテストを実行する共通ヘルパー
func runSimpleBotTest(t *testing.T, req *runSimpleBotTestParams) {
	runBotTest(t, &runBotTestParams{
		StatusCode:   req.StatusCode,
		ResponseBody: "",
		TestFunc:     req.TestFunc,
		ExpectError:  req.ExpectError,
		TestName:     req.TestName,
	})
}

// runBotTest HTTPクライアントのモック付きでボットテストを実行する共通ヘルパー
func runBotTest(t *testing.T, req *runBotTestParams) {
	t.Helper()
	mockClient := http.NewMockHTTPClient(req.StatusCode, req.ResponseBody)
	bot := misskey.NewBotWithClient(&misskey.BotSetting{
		Domain: "example.com",
		Token:  "token",
		Client: mockClient,
	})

	if err := req.TestFunc(bot); !errors.Is(err, req.ExpectError) {
		t.Errorf("%s error = %v, expectError = %v", req.TestName, err, req.ExpectError)
	}
}
