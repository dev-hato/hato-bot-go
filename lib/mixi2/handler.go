package mixi2

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	constv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/const/v1"
	modelv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/model/v1"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"hato-bot-go/lib"
	"hato-bot-go/lib/amesh"
	"hato-bot-go/lib/httpclient"
)

type HandlerSetting struct {
	Conn          *grpc.ClientConn
	Authenticator auth.Authenticator
	YahooAPIToken string
}

type uploadFileParams struct {
	description string
	buffer      *bytes.Buffer
}

// processAmeshCommandParams ameshコマンドの処理パラメータ
type processAmeshCommandParams struct {
	Place         string
	YahooAPIToken string
	PostID        string
	PostMask      *modelv1.PostMask
}

// Handler event.EventHandlerインターフェースを実装する
type Handler struct {
	APIClient     application_apiv1.ApplicationServiceClient
	Authenticator auth.Authenticator
	YahooAPIToken string
}

// NewHandler 新しいHandlerを作成する
func NewHandler(config *HandlerSetting) *Handler {
	return &Handler{
		APIClient:     application_apiv1.NewApplicationServiceClient(config.Conn),
		Authenticator: config.Authenticator,
		YahooAPIToken: config.YahooAPIToken,
	}
}

// uploadFile メディアファイルをアップロードし、メディアIDを返す
func (h *Handler) uploadFile(ctx context.Context, params *uploadFileParams) (mediaID string, err error) {
	contentType := http.DetectContentType(params.buffer.Bytes())
	bufLen := params.buffer.Len()

	if bufLen < 0 {
		return "", errors.New("buffer length is negative")
	}

	// アップロードの開始
	initiateResp, err := h.APIClient.InitiatePostMediaUpload(ctx, &application_apiv1.InitiatePostMediaUploadRequest{
		MediaType:   application_apiv1.InitiatePostMediaUploadRequest_TYPE_IMAGE,
		ContentType: contentType,
		DataSize:    uint64(bufLen),
		Description: &params.description,
	})
	if err != nil {
		return "", errors.Wrap(err, "Failed to APIClient.InitiatePostMediaUpload")
	}

	// メディアのアップロード
	req, err := http.NewRequest(http.MethodPost, initiateResp.UploadUrl, params.buffer)
	if err != nil {
		return "", errors.Wrap(err, "Failed to http.NewRequestWithContext")
	}

	// gRPCのアウトゴーイングメタデータからAuthorizationヘッダーを取り出してHTTPリクエストに設定する
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		for _, authorization := range md.Get("authorization") {
			req.Header.Set("Authorization", authorization)
		}
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "hato-bot-go/"+lib.Version)

	resp, err := httpclient.ExecuteHTTPRequest(&http.Client{Timeout: 30 * time.Second}, req)
	if err != nil {
		return "", errors.Wrap(err, "Failed to httpclient.ExecuteHTTPRequest")
	}
	defer func(body io.ReadCloser) {
		if closeErr := body.Close(); closeErr != nil {
			err = errors.Wrap(closeErr, "Failed to Close")
		}
	}(resp.Body)

	mediaID = initiateResp.GetMediaId()

	// 処理状況の確認
	for {
		statusResp, err := h.APIClient.GetPostMediaStatus(ctx, &application_apiv1.GetPostMediaStatusRequest{
			MediaId: mediaID,
		})
		if err != nil {
			return "", errors.Wrap(err, "Failed to APIClient.GetPostMediaStatus")
		}

		status := statusResp.GetStatus()

		if status == application_apiv1.GetPostMediaStatusResponse_STATUS_FAILED {
			return "", errors.New("media processing failed")
		}

		if status == application_apiv1.GetPostMediaStatusResponse_STATUS_COMPLETED {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return mediaID, nil
}

// processAmeshCommand ameshコマンドを処理
func (h *Handler) processAmeshCommand(ctx context.Context, authCtx context.Context, params *processAmeshCommandParams) error {
	if params == nil {
		return lib.ErrParamsNil
	}
	if params.PostID == "" {
		return lib.ErrParamsEmptyString
	}

	// 処理中リアクションを追加
	if _, err := h.APIClient.AddStampToPost(
		authCtx,
		&application_apiv1.AddStampToPostRequest{PostId: params.PostID, StampId: "o_eye"},
	); err != nil {
		return errors.Wrap(err, "Failed to APIClient.AddStampToPost")
	}

	// 位置を解析してログに出力
	location, err := amesh.ParseLocationWithLog(ctx, params.Place, params.YahooAPIToken)
	if err != nil {
		return errors.Wrap(err, "Failed to amesh.ParseLocationWithLog")
	}

	description := fmt.Sprintf("%s (%.4f, %.4f) の雨雲レーダー画像", location.PlaceName, location.Lat, location.Lng)

	// 画像をメモリ上に作成
	imageBuffer, err := amesh.CreateImageBuffer(ctx, location)
	if err != nil {
		return errors.Wrap(err, "Failed to amesh.CreateImageBuffer")
	}

	// mixi2にメモリから直接アップロード
	mediaID, err := h.uploadFile(authCtx, &uploadFileParams{
		description: description,
		buffer:      imageBuffer,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to uploadFile")
	}

	// 結果をポストとして投稿
	if _, err := h.APIClient.CreatePost(
		authCtx,
		&application_apiv1.CreatePostRequest{
			Text:            fmt.Sprintf("📡 %sだっぽ", description),
			MediaIdList:     []string{mediaID},
			InReplyToPostId: &params.PostID,
			PostMask:        params.PostMask,
		},
	); err != nil {
		return errors.Wrap(err, "Failed to APIClient.CreatePost")
	}

	log.Printf("Successfully processed amesh command for %s", location.PlaceName)
	return nil
}

// Handle mixi2からのイベントを処理する
func (h *Handler) Handle(ctx context.Context, event *modelv1.Event) error {
	if event.GetEventType() != constv1.EventType_EVENT_TYPE_POST_CREATED {
		return nil
	}

	postCreatedEvent := event.GetPostCreatedEvent()

	if postCreatedEvent == nil {
		return lib.ErrParamsNil
	}

	if !slices.Contains(postCreatedEvent.GetEventReasonList(), constv1.EventReason_EVENT_REASON_POST_MENTIONED) {
		return nil
	}

	post := postCreatedEvent.GetPost()

	if post == nil {
		return lib.ErrParamsNil
	}

	postID := post.GetPostId()

	if postID == "" {
		return lib.ErrParamsEmptyString
	}

	text := post.GetText()

	if text == "" {
		return lib.ErrParamsEmptyString
	}

	postMask := post.GetPostMask()

	if postMask != nil {
		postMask.Caption = "隠すっぽ！"
	}

	// ameshコマンドを解析
	parseResult := amesh.ParseAmeshCommand(text)

	if !parseResult.IsAmesh {
		return nil
	}

	log.Printf("Processing amesh command for place: %s", parseResult.Place)

	authCtx, err := h.Authenticator.AuthorizedContext(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to Authenticator.AuthorizedContext")
	}

	// ameshコマンドを処理
	if err := h.processAmeshCommand(ctx, authCtx, &processAmeshCommandParams{
		Place:         parseResult.Place,
		YahooAPIToken: h.YahooAPIToken,
		PostID:        postID,
		PostMask:      postMask,
	}); err != nil {
		log.Printf("Error processing amesh command: %v", err)

		// エラーメッセージを投稿
		if _, err := h.APIClient.CreatePost(
			authCtx,
			&application_apiv1.CreatePostRequest{
				Text:            "申し訳ないっぽ。ameshコマンドの処理中にエラーが発生したっぽ",
				InReplyToPostId: &postID,
				PostMask:        postMask,
			},
		); err != nil {
			return errors.Wrap(err, "Failed to APIClient.CreatePost")
		}
	}

	return nil
}
