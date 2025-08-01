package misskey

import (
	"net/http"
	"strings"
	"time"

	"hato-bot-go/lib/amesh"
)

// BotSetting Misskeyボットの設定
type BotSetting struct {
	Domain string       // Misskeyのドメイン
	Token  string       // APIトークン
	Client *http.Client // HTTPクライアント
}

// Note Misskeyのノート構造体
type Note struct {
	ID         string   `json:"id"`
	Text       string   `json:"text,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
	FileIDs    []string `json:"fileIds,omitempty"`
	ReplyID    string   `json:"replyId,omitempty"`
	CW         *string  `json:"cw,omitempty"`
	User       struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Host     string `json:"host,omitempty"`
	} `json:"user"`
}

// CreateNoteRequest ノート作成のリクエスト構造体
type CreateNoteRequest struct {
	Text         string   // ノートのテキスト
	FileIDs      []string // 添付ファイルのID一覧
	OriginalNote *Note    // 返信元のノート
}

// File アップロードされたファイルの構造体
type File struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ParseResult ameshコマンドの解析結果を表す構造体
type ParseResult struct {
	Place   string
	IsAmesh bool
}

type ProcessAmeshCommandRequest struct {
	Note          *Note
	Place         string
	YahooAPIToken string
}

// NewBotWithClient HTTPクライアント注入可能なBotインスタンスを作成
func NewBotWithClient(botSetting *BotSetting) *Bot {
	if botSetting == nil {
		return nil
	}
	if botSetting.Client == nil {
		return nil
	}
	return &Bot{
		BotSetting: botSetting,
		UserAgent:  "hato-bot-go/" + amesh.Version,
	}
}

// NewBot 新しいBotインスタンスを作成
func NewBot(domain, token string) *Bot {
	return NewBotWithClient(&BotSetting{
		Domain: domain,
		Token:  token,
		Client: &http.Client{Timeout: 30 * time.Second},
	})
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
