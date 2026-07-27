package clients

import (
	"testing"

	"github.com/spf13/viper"
)

func TestNewOptionsDefaultsToBBSServiceNames(t *testing.T) {
	o := NewOptions(viper.New())

	if o.Admin != "bbs-admin-service" {
		t.Fatalf("admin = %q", o.Admin)
	}
	if o.AdminInternalAuthToken != LocalDevAdminInternalAuthToken {
		t.Fatalf("admin internal auth token = %q", o.AdminInternalAuthToken)
	}
	if o.User != "bbs-user-service" {
		t.Fatalf("user = %q", o.User)
	}
	if o.UserInternalAuthToken != LocalDevUserInternalAuthToken {
		t.Fatalf("user internal auth token = %q", o.UserInternalAuthToken)
	}
	if o.ContentInternalAuthToken != LocalDevContentInternalAuthToken {
		t.Fatalf("content internal auth token = %q", o.ContentInternalAuthToken)
	}
	if o.CommentInternalAuthToken != LocalDevCommentInternalAuthToken {
		t.Fatalf("comment internal auth token = %q", o.CommentInternalAuthToken)
	}
	if o.MallInternalAuthToken != LocalDevMallInternalAuthToken {
		t.Fatalf("mall internal auth token = %q", o.MallInternalAuthToken)
	}
	if o.CreditInternalAuthToken != LocalDevCreditInternalAuthToken {
		t.Fatalf("credit internal auth token = %q", o.CreditInternalAuthToken)
	}
	if o.FileInternalAuthToken != LocalDevFileInternalAuthToken {
		t.Fatalf("file internal auth token = %q", o.FileInternalAuthToken)
	}
	if o.FeedInternalAuthToken != LocalDevFeedInternalAuthToken {
		t.Fatalf("feed internal auth token = %q", o.FeedInternalAuthToken)
	}
	if o.ReactionInternalAuthToken != LocalDevReactionInternalAuthToken {
		t.Fatalf("reaction internal auth token = %q", o.ReactionInternalAuthToken)
	}
	if o.SearchInternalAuthToken != LocalDevSearchInternalAuthToken {
		t.Fatalf("search internal auth token = %q", o.SearchInternalAuthToken)
	}
	if o.Notification != "bbs-notification-service" {
		t.Fatalf("notification = %q", o.Notification)
	}
	if o.NotificationInternalAuthToken != LocalDevNotificationInternalAuthToken {
		t.Fatalf("notification internal auth token = %q", o.NotificationInternalAuthToken)
	}
	if o.Chat != "bbs-chat-service" {
		t.Fatalf("chat = %q", o.Chat)
	}
	if o.ChatInternalAuthToken != LocalDevChatInternalAuthToken {
		t.Fatalf("chat internal auth token = %q", o.ChatInternalAuthToken)
	}
}

func TestNewOptionsNormalizesLegacyServiceNames(t *testing.T) {
	v := viper.New()
	v.Set("upstreams.admin", "admin-service")
	v.Set("upstreams.user", "user-service")
	v.Set("upstreams.notification", "notification-service")
	v.Set("upstreams.chat", "chat-service")

	o := NewOptions(v)

	if o.Admin != "bbs-admin-service" {
		t.Fatalf("admin = %q", o.Admin)
	}
	if o.User != "bbs-user-service" {
		t.Fatalf("user = %q", o.User)
	}
	if o.Notification != "bbs-notification-service" {
		t.Fatalf("notification = %q", o.Notification)
	}
	if o.Chat != "bbs-chat-service" {
		t.Fatalf("chat = %q", o.Chat)
	}
}

func TestNewOptionsPreservesCustomServiceNames(t *testing.T) {
	v := viper.New()
	v.Set("upstreams.admin", "custom-admin")
	v.Set("upstreams.adminInternalAuthToken", " custom-admin-token ")
	v.Set("upstreams.chat", " custom-chat ")
	v.Set("upstreams.chatInternalAuthToken", " custom-chat-token ")
	v.Set("upstreams.commentInternalAuthToken", " custom-comment-token ")
	v.Set("upstreams.contentInternalAuthToken", " custom-content-token ")
	v.Set("upstreams.userInternalAuthToken", " custom-user-token ")
	v.Set("upstreams.mallInternalAuthToken", " custom-mall-token ")
	v.Set("upstreams.creditInternalAuthToken", " custom-credit-token ")
	v.Set("upstreams.fileInternalAuthToken", " custom-file-token ")
	v.Set("upstreams.feedInternalAuthToken", " custom-feed-token ")
	v.Set("upstreams.notificationInternalAuthToken", " custom-notification-token ")
	v.Set("upstreams.reactionInternalAuthToken", " custom-reaction-token ")
	v.Set("upstreams.searchInternalAuthToken", " custom-search-token ")

	o := NewOptions(v)

	if o.Admin != "custom-admin" {
		t.Fatalf("admin = %q", o.Admin)
	}
	if o.AdminInternalAuthToken != "custom-admin-token" {
		t.Fatalf("admin internal auth token = %q", o.AdminInternalAuthToken)
	}
	if o.Chat != "custom-chat" {
		t.Fatalf("chat = %q", o.Chat)
	}
	if o.ChatInternalAuthToken != "custom-chat-token" {
		t.Fatalf("chat internal auth token = %q", o.ChatInternalAuthToken)
	}
	if o.CommentInternalAuthToken != "custom-comment-token" {
		t.Fatalf("comment internal auth token = %q", o.CommentInternalAuthToken)
	}
	if o.ContentInternalAuthToken != "custom-content-token" {
		t.Fatalf("content internal auth token = %q", o.ContentInternalAuthToken)
	}
	if o.UserInternalAuthToken != "custom-user-token" {
		t.Fatalf("user internal auth token = %q", o.UserInternalAuthToken)
	}
	if o.MallInternalAuthToken != "custom-mall-token" {
		t.Fatalf("mall internal auth token = %q", o.MallInternalAuthToken)
	}
	if o.CreditInternalAuthToken != "custom-credit-token" {
		t.Fatalf("credit internal auth token = %q", o.CreditInternalAuthToken)
	}
	if o.FileInternalAuthToken != "custom-file-token" {
		t.Fatalf("file internal auth token = %q", o.FileInternalAuthToken)
	}
	if o.FeedInternalAuthToken != "custom-feed-token" {
		t.Fatalf("feed internal auth token = %q", o.FeedInternalAuthToken)
	}
	if o.NotificationInternalAuthToken != "custom-notification-token" {
		t.Fatalf("notification internal auth token = %q", o.NotificationInternalAuthToken)
	}
	if o.ReactionInternalAuthToken != "custom-reaction-token" {
		t.Fatalf("reaction internal auth token = %q", o.ReactionInternalAuthToken)
	}
	if o.SearchInternalAuthToken != "custom-search-token" {
		t.Fatalf("search internal auth token = %q", o.SearchInternalAuthToken)
	}
}

func TestNewOptionsResolvesPerUpstreamInternalAuthTransport(t *testing.T) {
	v := viper.New()
	v.Set("grpc.client.secure", true) // Legacy default for unconfigured upstreams.
	v.Set("upstreams.adminInternalAuthSecure", false)
	v.Set("upstreams.chatInternalAuthSecure", true)

	o := NewOptions(v)

	if o.AdminInternalAuthSecure {
		t.Fatal("admin should honor its explicit plaintext setting")
	}
	if !o.ChatInternalAuthSecure {
		t.Fatal("chat should honor its explicit mTLS setting")
	}
	if !o.UserInternalAuthSecure || !o.MallInternalAuthSecure || !o.CreditInternalAuthSecure || !o.FileInternalAuthSecure {
		t.Fatalf("unconfigured upstreams should retain the legacy secure default: %#v", o)
	}
}
