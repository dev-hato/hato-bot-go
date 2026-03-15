package main

import (
	"context"
	"crypto/tls"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cockroachdb/errors"
	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	"github.com/mixigroup/mixi2-application-sdk-go/event/stream"
	application_streamv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_stream/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"hato-bot-go/lib"
	"hato-bot-go/lib/mixi2"
)

// run ボットのメイン処理を実行し、エラーを返す
func run() (err error) {
	// 環境変数から設定を取得
	streamAddress := os.Getenv("MIXI2_STREAM_ADDRESS")
	clientID := os.Getenv("MIXI2_CLIENT_ID")
	clientSecret := os.Getenv("MIXI2_CLIENT_SECRET")
	tokenURL := os.Getenv("MIXI2_TOKEN_URL")
	apiAddress := os.Getenv("MIXI2_API_ADDRESS")

	if streamAddress == "" || clientID == "" || clientSecret == "" || tokenURL == "" || apiAddress == "" {
		return errors.New("MIXI2_STREAM_ADDRESS, MIXI2_CLIENT_ID, MIXI2_CLIENT_SECRET, MIXI2_TOKEN_URL and MIXI2_API_ADDRESS environment variables must be set")
	}

	yahooAPIToken := os.Getenv("YAHOO_API_TOKEN")

	// Yahoo APIキーも必要
	if yahooAPIToken == "" {
		return errors.New("YAHOO_API_TOKEN environment variable must be set")
	}

	// HTTPサーバーを別ゴルーチンで開始
	go lib.StartStatusHTTPServer()

	withTransportCredentials := grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS13,
	}))

	// gRPCストリーム接続確立
	streamConn, err := grpc.NewClient(streamAddress, withTransportCredentials)
	if err != nil {
		return errors.Wrap(err, "Failed to grpc.NewClient")
	}
	defer func(streamConn *grpc.ClientConn) {
		if closeErr := streamConn.Close(); closeErr != nil {
			err = errors.Wrap(err, "Failed to Close")
		}
	}(streamConn)

	// gRPC API接続確立
	apiConn, err := grpc.NewClient(apiAddress, withTransportCredentials)
	if err != nil {
		return errors.Wrap(err, "Failed to grpc.NewClient")
	}
	defer func(apiConn *grpc.ClientConn) {
		if closeErr := apiConn.Close(); closeErr != nil {
			err = errors.Wrap(err, "Failed to Close")
		}
	}(apiConn)

	log.Println("hato-bot-go started")

	// 認証クライアント作成
	authenticator, err := auth.NewAuthenticator(clientID, clientSecret, tokenURL)
	if err != nil {
		return errors.Wrap(err, "Failed to auth.NewAuthenticator")
	}

	// グレースフルシャットダウン設定
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down...")
		cancel()
	}()

	log.Printf("starting stream watcher: address=%s\n", streamAddress)

	// 監視開始
	if err := stream.NewStreamWatcher(
		application_streamv1.NewApplicationServiceClient(streamConn),
		authenticator,
	).Watch(ctx, mixi2.NewHandler(&mixi2.HandlerSetting{
		Conn:          apiConn,
		Authenticator: authenticator,
		YahooAPIToken: yahooAPIToken,
	})); err != nil && !errors.Is(err, context.Canceled) {
		return errors.Wrap(err, "Failed to Watch")
	}

	log.Println("stopped")
	return nil
}

// main mixi2ボットとして実行
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
