package upstream

import (
	"context"
	"strings"
	"testing"
)

func TestInternalAuthCredentials(t *testing.T) {
	credentials := internalAuthCredentials{token: "notification-internal-token"}

	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[internalAuthMetadataKey]; got != "notification-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("internal credential must support the configured local insecure transport")
	}
}

func TestNewRequiresInternalAuthTokensBeforeDialing(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{
			name: "missing user token",
			opts: Options{
				ReactionInternalAuthToken:     "reaction-token",
				NotificationInternalAuthToken: "notification-token",
				SearchInternalAuthToken:       "search-token",
				CommentInternalAuthToken:      "comment-token",
			},
			want: "user internal auth token required",
		},
		{
			name: "blank reaction token",
			opts: Options{
				UserInternalAuthToken:         " user-token ",
				ReactionInternalAuthToken:     "  ",
				NotificationInternalAuthToken: "notification-token",
				SearchInternalAuthToken:       "search-token",
				CommentInternalAuthToken:      "comment-token",
			},
			want: "reaction internal auth token required",
		},
		{
			name: "blank notification token",
			opts: Options{
				UserInternalAuthToken:         " user-token ",
				ReactionInternalAuthToken:     " reaction-token ",
				NotificationInternalAuthToken: "  ",
				SearchInternalAuthToken:       "search-token",
				CommentInternalAuthToken:      "comment-token",
			},
			want: "notification internal auth token required",
		},
		{
			name: "blank search token",
			opts: Options{
				UserInternalAuthToken:         " user-token ",
				ReactionInternalAuthToken:     " reaction-token ",
				NotificationInternalAuthToken: " notification-token ",
				SearchInternalAuthToken:       "  ",
				CommentInternalAuthToken:      "comment-token",
			},
			want: "search internal auth token required",
		},
		{
			name: "blank comment token",
			opts: Options{
				UserInternalAuthToken:         " user-token ",
				ReactionInternalAuthToken:     " reaction-token ",
				NotificationInternalAuthToken: " notification-token ",
				SearchInternalAuthToken:       " search-token ",
				CommentInternalAuthToken:      "  ",
			},
			want: "comment internal auth token required",
		},
		{
			name: "blank content token",
			opts: Options{
				UserInternalAuthToken:         " user-token ",
				ReactionInternalAuthToken:     " reaction-token ",
				NotificationInternalAuthToken: " notification-token ",
				SearchInternalAuthToken:       " search-token ",
				CommentInternalAuthToken:      " comment-token ",
				ContentInternalAuthToken:      "  ",
			},
			want: "content internal auth token required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(nil, tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("New() error = %v, want %q", err, tt.want)
			}
		})
	}
}
