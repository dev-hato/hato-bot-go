package misskey_test

import (
	"testing"

	"github.com/cockroachdb/errors"

	"hato-bot-go/lib/httpclient"
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
	mockClient := httpclient.NewMockHTTPClient(req.StatusCode, req.ResponseBody)
	bot := misskey.NewBotWithClient(&misskey.BotSetting{
		Domain: "example.com",
		Token:  "token",
		Client: mockClient,
	})

	if err := req.TestFunc(bot); !errors.Is(err, req.ExpectError) {
		t.Errorf("%s error = %v, expectError = %v", req.TestName, err, req.ExpectError)
	}
}
