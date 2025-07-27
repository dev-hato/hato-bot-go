package misskey

import "strings"

// Note Misskeyのノート構造体
type Note struct {
	ID         string   `json:"id"`
	Text       string   `json:"text,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
	FileIDs    []string `json:"fileIds,omitempty"`
	ReplyID    string   `json:"replyId,omitempty"`
	CW         *string  `json:"cw,omitempty"`
	User       User     `json:"user"`
}

// User Misskeyのユーザー構造体
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Host     string `json:"host,omitempty"`
}

// ParseResult ameshコマンドの解析結果を表す構造体
type ParseResult struct {
	Place   string
	IsAmesh bool
}

// ParseAmeshCommand ameshコマンドを解析
func ParseAmeshCommand(text string) ParseResult {
	// メンションを除去
	text = strings.TrimSpace(text)

	// @username を削除
	words := strings.Fields(text)
	var cleanWords []string
	for _, word := range words {
		if !strings.HasPrefix(word, "@") {
			cleanWords = append(cleanWords, word)
		}
	}
	text = strings.Join(cleanWords, " ")

	// ameshコマンドかチェック
	if strings.HasPrefix(text, "amesh ") {
		place := strings.TrimPrefix(text, "amesh ")
		return ParseResult{
			Place:   strings.TrimSpace(place),
			IsAmesh: true,
		}
	}

	if text == "amesh" {
		return ParseResult{
			Place:   "東京", // デフォルトの場所
			IsAmesh: true,
		}
	}

	return ParseResult{
		Place:   "",
		IsAmesh: false,
	}
}
