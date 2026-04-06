package mixi2

import (
	"testing"

	"github.com/cockroachdb/errors"
	constv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/const/v1"
	modelv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/model/v1"
	apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"
	"go.uber.org/mock/gomock"

	"hato-bot-go/lib"
)

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
		makeHandler func(t *testing.T) *Handler
		ev          *modelv1.Event
		expectError error
	}{
		{
			name: "POST_CREATED以外のイベント",
			makeHandler: func(_ *testing.T) *Handler {
				return &Handler{}
			},
			ev:          &modelv1.Event{EventType: constv1.EventType_EVENT_TYPE_PING},
			expectError: nil,
		},
		{
			name: "PostCreatedEventがnil",
			makeHandler: func(_ *testing.T) *Handler {
				return &Handler{}
			},
			ev: &modelv1.Event{
				EventType: constv1.EventType_EVENT_TYPE_POST_CREATED,
			},
			expectError: lib.ErrParamsNil,
		},
		{
			name: "MENTIONED以外のEventReason",
			makeHandler: func(_ *testing.T) *Handler {
				return &Handler{}
			},
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
			name: "Postがnil",
			makeHandler: func(_ *testing.T) *Handler {
				return &Handler{}
			},
			ev:          mentionedPostCreatedEvent(nil),
			expectError: lib.ErrParamsNil,
		},
		{
			name: "postIDが空",
			makeHandler: func(_ *testing.T) *Handler {
				return &Handler{}
			},
			ev:          mentionedPostCreatedEvent(&modelv1.Post{PostId: "", Text: "amesh 東京"}),
			expectError: lib.ErrParamsEmptyString,
		},
		{
			name: "textが空",
			makeHandler: func(_ *testing.T) *Handler {
				return &Handler{}
			},
			ev:          mentionedPostCreatedEvent(&modelv1.Post{PostId: "post123", Text: ""}),
			expectError: lib.ErrParamsEmptyString,
		},
		{
			name: "amesh以外のコマンド",
			makeHandler: func(_ *testing.T) *Handler {
				return &Handler{}
			},
			ev:          mentionedPostCreatedEvent(&modelv1.Post{PostId: "post123", Text: "こんにちは"}),
			expectError: nil,
		},
		{
			name: "ameshコマンドで認証に失敗",
			makeHandler: func(t *testing.T) *Handler {
				mockAuth := NewMockAuthenticator(gomock.NewController(t))
				mockAuth.EXPECT().
					AuthorizedContext(t.Context()).
					Return(nil, errAuthFailed)
				return &Handler{
					Authenticator: mockAuth,
				}
			},
			ev:          mentionedPostCreatedEvent(&modelv1.Post{PostId: "post123", Text: "amesh 東京"}),
			expectError: errAuthFailed,
		},
		{
			name: "ameshコマンドでAddStampToPostが失敗してもエラーメッセージを投稿して正常終了",
			makeHandler: func(t *testing.T) *Handler {
				ctrl := gomock.NewController(t)
				mockAuth := NewMockAuthenticator(ctrl)
				mockClient := NewMockApplicationServiceClient(ctrl)
				ctx := t.Context()
				postID := "post123"
				mockAuth.EXPECT().
					AuthorizedContext(ctx).
					Return(ctx, nil)
				mockClient.EXPECT().
					AddStampToPost(ctx, &apiv1.AddStampToPostRequest{
						PostId:  postID,
						StampId: "o_eye",
					}).
					Return(nil, errors.New("スタンプ追加エラー"))
				mockClient.EXPECT().
					CreatePost(ctx, &apiv1.CreatePostRequest{
						Text:            "申し訳ないっぽ。ameshコマンドの処理中にエラーが発生したっぽ",
						InReplyToPostId: &postID,
					}).
					Return(&apiv1.CreatePostResponse{}, nil)
				return &Handler{
					Authenticator: mockAuth,
					APIClient:     mockClient,
					YahooAPIToken: "",
				}
			},
			ev:          mentionedPostCreatedEvent(&modelv1.Post{PostId: "post123", Text: "amesh 東京"}),
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := tt.makeHandler(t)
			if err := handler.Handle(t.Context(), tt.ev); !errors.Is(err, tt.expectError) {
				t.Errorf("Handle() error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}
