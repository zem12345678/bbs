package clients

import (
	"strings"

	"github.com/spf13/viper"
)

const (
	LocalDevAdminInternalAuthToken                 = "bbs-local-admin-internal-token"
	MinimumProductionAdminInternalAuthBytes        = 32
	LocalDevChatInternalAuthToken                  = "bbs-local-chat-internal-token"
	MinimumProductionChatInternalAuthBytes         = 32
	LocalDevCommentInternalAuthToken               = "bbs-local-comment-internal-token"
	MinimumProductionCommentInternalAuthBytes      = 32
	LocalDevContentInternalAuthToken               = "bbs-local-content-internal-token"
	MinimumProductionContentInternalAuthBytes      = 32
	LocalDevUserInternalAuthToken                  = "bbs-local-user-internal-token"
	MinimumProductionUserInternalAuthBytes         = 32
	LocalDevMallInternalAuthToken                  = "bbs-local-mall-internal-token"
	MinimumProductionMallInternalAuthBytes         = 32
	LocalDevCreditInternalAuthToken                = "bbs-local-credit-internal-token"
	MinimumProductionCreditInternalAuthBytes       = 32
	LocalDevFileInternalAuthToken                  = "bbs-local-file-internal-token"
	MinimumProductionFileInternalAuthBytes         = 32
	LocalDevFeedInternalAuthToken                  = "bbs-local-feed-internal-token"
	MinimumProductionFeedInternalAuthBytes         = 32
	LocalDevNotificationInternalAuthToken          = "bbs-local-notification-internal-token"
	MinimumProductionNotificationInternalAuthBytes = 32
	LocalDevReactionInternalAuthToken              = "bbs-local-reaction-internal-token"
	MinimumProductionReactionInternalAuthBytes     = 32
	LocalDevSearchInternalAuthToken                = "bbs-local-search-internal-token"
	MinimumProductionSearchInternalAuthBytes       = 32
)

type Options struct {
	Admin                         string
	AdminInternalAuthToken        string
	AdminInternalAuthSecure       bool
	User                          string
	UserInternalAuthToken         string
	UserInternalAuthSecure        bool
	Content                       string
	ContentInternalAuthToken      string
	Comment                       string
	CommentInternalAuthToken      string
	Reaction                      string
	ReactionInternalAuthToken     string
	Search                        string
	SearchInternalAuthToken       string
	Feed                          string
	FeedInternalAuthToken         string
	Credit                        string
	CreditInternalAuthToken       string
	CreditInternalAuthSecure      bool
	Mall                          string
	MallInternalAuthToken         string
	MallInternalAuthSecure        bool
	Notification                  string
	NotificationInternalAuthToken string
	File                          string
	FileInternalAuthToken         string
	FileInternalAuthSecure        bool
	Chat                          string
	ChatInternalAuthToken         string
	ChatInternalAuthSecure        bool
}

func NewOptions(v *viper.Viper) Options {
	o := Options{
		Admin:                         v.GetString("upstreams.admin"),
		AdminInternalAuthToken:        v.GetString("upstreams.adminInternalAuthToken"),
		AdminInternalAuthSecure:       upstreamInternalAuthSecure(v, "upstreams.adminInternalAuthSecure"),
		User:                          v.GetString("upstreams.user"),
		UserInternalAuthToken:         v.GetString("upstreams.userInternalAuthToken"),
		UserInternalAuthSecure:        upstreamInternalAuthSecure(v, "upstreams.userInternalAuthSecure"),
		Content:                       v.GetString("upstreams.content"),
		ContentInternalAuthToken:      v.GetString("upstreams.contentInternalAuthToken"),
		Comment:                       v.GetString("upstreams.comment"),
		CommentInternalAuthToken:      v.GetString("upstreams.commentInternalAuthToken"),
		Reaction:                      v.GetString("upstreams.reaction"),
		ReactionInternalAuthToken:     v.GetString("upstreams.reactionInternalAuthToken"),
		Search:                        v.GetString("upstreams.search"),
		SearchInternalAuthToken:       v.GetString("upstreams.searchInternalAuthToken"),
		Feed:                          v.GetString("upstreams.feed"),
		FeedInternalAuthToken:         v.GetString("upstreams.feedInternalAuthToken"),
		Credit:                        v.GetString("upstreams.credit"),
		CreditInternalAuthToken:       v.GetString("upstreams.creditInternalAuthToken"),
		CreditInternalAuthSecure:      upstreamInternalAuthSecure(v, "upstreams.creditInternalAuthSecure"),
		Mall:                          v.GetString("upstreams.mall"),
		MallInternalAuthToken:         v.GetString("upstreams.mallInternalAuthToken"),
		MallInternalAuthSecure:        upstreamInternalAuthSecure(v, "upstreams.mallInternalAuthSecure"),
		Notification:                  v.GetString("upstreams.notification"),
		NotificationInternalAuthToken: v.GetString("upstreams.notificationInternalAuthToken"),
		File:                          v.GetString("upstreams.file"),
		FileInternalAuthToken:         v.GetString("upstreams.fileInternalAuthToken"),
		FileInternalAuthSecure:        upstreamInternalAuthSecure(v, "upstreams.fileInternalAuthSecure"),
		Chat:                          v.GetString("upstreams.chat"),
		ChatInternalAuthToken:         v.GetString("upstreams.chatInternalAuthToken"),
		ChatInternalAuthSecure:        upstreamInternalAuthSecure(v, "upstreams.chatInternalAuthSecure"),
	}
	o.applyDefaults()
	return o
}

// upstreamInternalAuthSecure supports a staged migration away from the
// historical grpc.client.secure switch. Each protected upstream may now opt
// into mTLS independently; deployments without an explicit upstream setting
// retain the previous global behavior.
func upstreamInternalAuthSecure(v *viper.Viper, key string) bool {
	if v.IsSet(key) {
		return v.GetBool(key)
	}
	return v.GetBool("grpc.client.secure")
}

func (o *Options) applyDefaults() {
	o.Admin = serviceNameOrDefault(o.Admin, "bbs-admin-service")
	o.AdminInternalAuthToken = strings.TrimSpace(o.AdminInternalAuthToken)
	if o.AdminInternalAuthToken == "" {
		o.AdminInternalAuthToken = LocalDevAdminInternalAuthToken
	}
	o.User = serviceNameOrDefault(o.User, "bbs-user-service")
	o.UserInternalAuthToken = strings.TrimSpace(o.UserInternalAuthToken)
	if o.UserInternalAuthToken == "" {
		o.UserInternalAuthToken = LocalDevUserInternalAuthToken
	}
	o.Content = serviceNameOrDefault(o.Content, "bbs-content-service")
	o.ContentInternalAuthToken = strings.TrimSpace(o.ContentInternalAuthToken)
	if o.ContentInternalAuthToken == "" {
		o.ContentInternalAuthToken = LocalDevContentInternalAuthToken
	}
	o.Comment = serviceNameOrDefault(o.Comment, "bbs-comment-service")
	o.CommentInternalAuthToken = strings.TrimSpace(o.CommentInternalAuthToken)
	if o.CommentInternalAuthToken == "" {
		o.CommentInternalAuthToken = LocalDevCommentInternalAuthToken
	}
	o.Reaction = serviceNameOrDefault(o.Reaction, "bbs-reaction-service")
	o.ReactionInternalAuthToken = strings.TrimSpace(o.ReactionInternalAuthToken)
	if o.ReactionInternalAuthToken == "" {
		o.ReactionInternalAuthToken = LocalDevReactionInternalAuthToken
	}
	o.Search = serviceNameOrDefault(o.Search, "bbs-search-service")
	o.SearchInternalAuthToken = strings.TrimSpace(o.SearchInternalAuthToken)
	if o.SearchInternalAuthToken == "" {
		o.SearchInternalAuthToken = LocalDevSearchInternalAuthToken
	}
	o.Feed = serviceNameOrDefault(o.Feed, "bbs-feed-service")
	o.FeedInternalAuthToken = strings.TrimSpace(o.FeedInternalAuthToken)
	if o.FeedInternalAuthToken == "" {
		o.FeedInternalAuthToken = LocalDevFeedInternalAuthToken
	}
	o.Credit = serviceNameOrDefault(o.Credit, "bbs-credit-service")
	o.CreditInternalAuthToken = strings.TrimSpace(o.CreditInternalAuthToken)
	if o.CreditInternalAuthToken == "" {
		o.CreditInternalAuthToken = LocalDevCreditInternalAuthToken
	}
	o.Mall = serviceNameOrDefault(o.Mall, "bbs-mall-service")
	o.MallInternalAuthToken = strings.TrimSpace(o.MallInternalAuthToken)
	if o.MallInternalAuthToken == "" {
		o.MallInternalAuthToken = LocalDevMallInternalAuthToken
	}
	o.Notification = serviceNameOrDefault(o.Notification, "bbs-notification-service")
	o.NotificationInternalAuthToken = strings.TrimSpace(o.NotificationInternalAuthToken)
	if o.NotificationInternalAuthToken == "" {
		o.NotificationInternalAuthToken = LocalDevNotificationInternalAuthToken
	}
	o.File = serviceNameOrDefault(o.File, "bbs-file-service")
	o.FileInternalAuthToken = strings.TrimSpace(o.FileInternalAuthToken)
	if o.FileInternalAuthToken == "" {
		o.FileInternalAuthToken = LocalDevFileInternalAuthToken
	}
	o.Chat = serviceNameOrDefault(o.Chat, "bbs-chat-service")
	o.ChatInternalAuthToken = strings.TrimSpace(o.ChatInternalAuthToken)
	if o.ChatInternalAuthToken == "" {
		o.ChatInternalAuthToken = LocalDevChatInternalAuthToken
	}
}

func serviceNameOrDefault(value string, fallback string) string {
	value = normalizeServiceName(value)
	if value == "" {
		return fallback
	}
	return value
}

func normalizeServiceName(value string) string {
	switch strings.TrimSpace(value) {
	case "admin-service":
		return "bbs-admin-service"
	case "user-service":
		return "bbs-user-service"
	case "content-service":
		return "bbs-content-service"
	case "comment-service":
		return "bbs-comment-service"
	case "reaction-service":
		return "bbs-reaction-service"
	case "search-service":
		return "bbs-search-service"
	case "feed-service":
		return "bbs-feed-service"
	case "credit-service":
		return "bbs-credit-service"
	case "mall-service":
		return "bbs-mall-service"
	case "notification-service":
		return "bbs-notification-service"
	case "file-service":
		return "bbs-file-service"
	case "chat-service":
		return "bbs-chat-service"
	default:
		return strings.TrimSpace(value)
	}
}
