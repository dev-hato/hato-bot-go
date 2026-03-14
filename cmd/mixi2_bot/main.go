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

// newGRPCConn gRPC接続を確立し、接続とクローズ関数を返す
func newGRPCConn(address string) (*grpc.ClientConn, func(), error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to grpc.NewClient")
	}

	return conn, func() {
		if closeErr := conn.Close(); closeErr != nil {
			log.Fatal(errors.Wrap(closeErr, "Failed to Close"))
		}
	}, nil
}

// main mixi2ボットとして実行
func main() {
	// 環境変数から設定を取得
	streamAddress := os.Getenv("MIXI2_STREAM_ADDRESS")
	clientID := os.Getenv("MIXI2_CLIENT_ID")
	clientSecret := os.Getenv("MIXI2_CLIENT_SECRET")
	tokenURL := os.Getenv("MIXI2_TOKEN_URL")
	apiAddress := os.Getenv("MIXI2_API_ADDRESS")

	if streamAddress == "" || clientID == "" || clientSecret == "" || tokenURL == "" || apiAddress == "" {
		log.Fatal("MIXI2_STREAM_ADDRESS, MIXI2_CLIENT_ID, MIXI2_CLIENT_SECRET, MIXI2_TOKEN_URL and MIXI2_API_ADDRESS environment variables must be set")
	}

	yahooAPIToken := os.Getenv("YAHOO_API_TOKEN")

	// Yahoo APIキーも必要
	if yahooAPIToken == "" {
		log.Fatal("YAHOO_API_TOKEN environment variable must be set")
	}

	// HTTPサーバーを別ゴルーチンで開始
	go lib.StartStatusHTTPServer()

	// gRPCストリーム接続確立
	streamConn, streamClose, err := newGRPCConn(streamAddress)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Failed to grpc.NewClient"))
	}
	defer streamClose()

	// gRPC API接続確立
	apiConn, apiFunc, err := newGRPCConn(apiAddress)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Failed to grpc.NewClient"))
	}
	defer apiFunc()

	log.Println("hato-bot-go started")

	// 認証クライアント作成
	authenticator, err := auth.NewAuthenticator(clientID, clientSecret, tokenURL)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Failed to create authenticator"))
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

	// 監視開始
	if err := stream.NewStreamWatcher(
		application_streamv1.NewApplicationServiceClient(streamConn),
		authenticator,
	).Watch(ctx, mixi2.NewHandler(&mixi2.HandlerSetting{
		Conn:          apiConn,
		Authenticator: authenticator,
		YahooAPIToken: yahooAPIToken,
	})); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(errors.Wrap(err, "Failed to Watch"))
	}

	log.Println("stopped")
}
