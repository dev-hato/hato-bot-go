package mixi2

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	constv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/const/v1"
	modelv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/model/v1"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"
	"google.golang.org/grpc"

	"hato-bot-go/lib"
)

// mockAPIClient ApplicationServiceClientのモック
type mockAPIClient struct {
	getUsersFunc                func(ctx context.Context, in *application_apiv1.GetUsersRequest, opts ...grpc.CallOption) (*application_apiv1.GetUsersResponse, error)
	getPostsFunc                func(ctx context.Context, in *application_apiv1.GetPostsRequest, opts ...grpc.CallOption) (*application_apiv1.GetPostsResponse, error)
	createPostFunc              func(ctx context.Context, in *application_apiv1.CreatePostRequest, opts ...grpc.CallOption) (*application_apiv1.CreatePostResponse, error)
	initiatePostMediaUploadFunc func(ctx context.Context, in *application_apiv1.InitiatePostMediaUploadRequest, opts ...grpc.CallOption) (*application_apiv1.InitiatePostMediaUploadResponse, error)
	getPostMediaStatusFunc      func(ctx context.Context, in *application_apiv1.GetPostMediaStatusRequest, opts ...grpc.CallOption) (*application_apiv1.GetPostMediaStatusResponse, error)
	sendChatMessageFunc         func(ctx context.Context, in *application_apiv1.SendChatMessageRequest, opts ...grpc.CallOption) (*application_apiv1.SendChatMessageResponse, error)
	getStampsFunc               func(ctx context.Context, in *application_apiv1.GetStampsRequest, opts ...grpc.CallOption) (*application_apiv1.GetStampsResponse, error)
	addStampToPostFunc          func(ctx context.Context, in *application_apiv1.AddStampToPostRequest, opts ...grpc.CallOption) (*application_apiv1.AddStampToPostResponse, error)
}

func (m *mockAPIClient) GetUsers(ctx context.Context, in *application_apiv1.GetUsersRequest, opts ...grpc.CallOption) (*application_apiv1.GetUsersResponse, error) {
	if m.getUsersFunc != nil {
		return m.getUsersFunc(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAPIClient) GetPosts(ctx context.Context, in *application_apiv1.GetPostsRequest, opts ...grpc.CallOption) (*application_apiv1.GetPostsResponse, error) {
	if m.getPostsFunc != nil {
		return m.getPostsFunc(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAPIClient) CreatePost(ctx context.Context, in *application_apiv1.CreatePostRequest, opts ...grpc.CallOption) (*application_apiv1.CreatePostResponse, error) {
	if m.createPostFunc != nil {
		return m.createPostFunc(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAPIClient) InitiatePostMediaUpload(ctx context.Context, in *application_apiv1.InitiatePostMediaUploadRequest, opts ...grpc.CallOption) (*application_apiv1.InitiatePostMediaUploadResponse, error) {
	if m.initiatePostMediaUploadFunc != nil {
		return m.initiatePostMediaUploadFunc(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAPIClient) GetPostMediaStatus(ctx context.Context, in *application_apiv1.GetPostMediaStatusRequest, opts ...grpc.CallOption) (*application_apiv1.GetPostMediaStatusResponse, error) {
	if m.getPostMediaStatusFunc != nil {
		return m.getPostMediaStatusFunc(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAPIClient) SendChatMessage(ctx context.Context, in *application_apiv1.SendChatMessageRequest, opts ...grpc.CallOption) (*application_apiv1.SendChatMessageResponse, error) {
	if m.sendChatMessageFunc != nil {
		return m.sendChatMessageFunc(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAPIClient) GetStamps(ctx context.Context, in *application_apiv1.GetStampsRequest, opts ...grpc.CallOption) (*application_apiv1.GetStampsResponse, error) {
	if m.getStampsFunc != nil {
		return m.getStampsFunc(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockAPIClient) AddStampToPost(ctx context.Context, in *application_apiv1.AddStampToPostRequest, opts ...grpc.CallOption) (*application_apiv1.AddStampToPostResponse, error) {
	if m.addStampToPostFunc != nil {
		return m.addStampToPostFunc(ctx, in, opts...)
	}
	return nil, nil
}

// mockAuthenticator auth.Authenticatorのモック
type mockAuthenticator struct {
	authorizedContextFunc func(ctx context.Context) (context.Context, error)
	getAccessTokenFunc    func(ctx context.Context) (string, error)
}

func (m *mockAuthenticator) GetAccessToken(ctx context.Context) (string, error) {
	if m.getAccessTokenFunc != nil {
		return m.getAccessTokenFunc(ctx)
	}
	return "", nil
}

func (m *mockAuthenticator) AuthorizedContext(ctx context.Context) (context.Context, error) {
	if m.authorizedContextFunc != nil {
		return m.authorizedContextFunc(ctx)
	}
	return ctx, nil
}

// インターフェースを実装していることをコンパイル時に確認する
var _ application_apiv1.ApplicationServiceClient = (*mockAPIClient)(nil)
var _ auth.Authenticator = (*mockAuthenticator)(nil)

// mentionedPostCreatedEvent メンション付きPOST_CREATEDイベントを作成するヘルパー
func mentionedPostCreatedEvent(post *modelv1.Post) *modelv1.Event {
	return &modelv1.Event{
		EventType: constv1.EventType_EVENT_TYPE_POST_CREATED,
		Body: &modelv1.Event_PostCreatedEvent{
			PostCreatedEvent: &modelv1.PostCreatedEvent{
				EventReasonList: []constv1.EventReason{constv1.EventReason_EVENT_REASON_POST_MENTIONED},
				Post:            post,
			},
		},
	}
}

func TestHandle(t *testing.T) {
	errAuthFailed := errors.New("認証に失敗しました")

	tests := []struct {
		name        string
		handler     *Handler
		ev          *modelv1.Event
		expectError error
	}{
		{
			name:        "POST_CREATED以外のイベント",
			handler:     &Handler{},
			ev:          &modelv1.Event{EventType: constv1.EventType_EVENT_TYPE_PING},
			expectError: nil,
		},
		{
			name:    "PostCreatedEventがnil",
			handler: &Handler{},
			ev: &modelv1.Event{
				EventType: constv1.EventType_EVENT_TYPE_POST_CREATED,
			},
			expectError: lib.ErrParamsNil,
		},
		{
			name:    "MENTIONED以外のEventReason",
			handler: &Handler{},
			ev: &modelv1.Event{
				EventType: constv1.EventType_EVENT_TYPE_POST_CREATED,
				Body: &modelv1.Event_PostCreatedEvent{
					PostCreatedEvent: &modelv1.PostCreatedEvent{
						EventReasonList: []constv1.EventReason{constv1.EventReason_EVENT_REASON_POST_REPLY},
					},
				},
			},
			expectError: nil,
		},
		{
			name:        "Postがnil",
			handler:     &Handler{},
			ev:          mentionedPostCreatedEvent(nil),
			expectError: lib.ErrParamsNil,
		},
		{
			name:        "postIDが空",
			handler:     &Handler{},
			ev:          mentionedPostCreatedEvent(&modelv1.Post{PostId: "", Text: "amesh 東京"}),
			expectError: lib.ErrParamsEmptyString,
		},
		{
			name:        "textが空",
			handler:     &Handler{},
			ev:          mentionedPostCreatedEvent(&modelv1.Post{PostId: "post123", Text: ""}),
			expectError: lib.ErrParamsEmptyString,
		},
		{
			name:        "amesh以外のコマンド",
			handler:     &Handler{},
			ev:          mentionedPostCreatedEvent(&modelv1.Post{PostId: "post123", Text: "こんにちは"}),
			expectError: nil,
		},
		{
			name: "ameshコマンドで認証に失敗",
			handler: &Handler{
				Authenticator: &mockAuthenticator{
					authorizedContextFunc: func(_ context.Context) (context.Context, error) {
						return nil, errAuthFailed
					},
				},
			},
			ev:          mentionedPostCreatedEvent(&modelv1.Post{PostId: "post123", Text: "amesh 東京"}),
			expectError: errAuthFailed,
		},
		{
			name: "ameshコマンドでAddStampToPostが失敗してもエラーメッセージを投稿して正常終了",
			handler: &Handler{
				Authenticator: &mockAuthenticator{},
				APIClient: &mockAPIClient{
					addStampToPostFunc: func(_ context.Context, _ *application_apiv1.AddStampToPostRequest, _ ...grpc.CallOption) (*application_apiv1.AddStampToPostResponse, error) {
						return nil, errors.New("スタンプ追加エラー")
					},
					createPostFunc: func(_ context.Context, _ *application_apiv1.CreatePostRequest, _ ...grpc.CallOption) (*application_apiv1.CreatePostResponse, error) {
						return &application_apiv1.CreatePostResponse{}, nil
					},
				},
				YahooAPIToken: "",
			},
			ev:          mentionedPostCreatedEvent(&modelv1.Post{PostId: "post123", Text: "amesh 東京"}),
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.handler.Handle(t.Context(), tt.ev); !errors.Is(err, tt.expectError) {
				t.Errorf("Handle() error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}
