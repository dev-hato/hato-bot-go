package misskey_test

import (
	"github.com/massongit/hato-bot-go/lib/misskey"
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
