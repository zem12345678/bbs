package clients

import (
	"strings"

	"github.com/spf13/viper"
)

type Options struct {
	Admin        string
	User         string
	Content      string
	Comment      string
	Reaction     string
	Search       string
	Feed         string
	Credit       string
	Mall         string
	Notification string
}

func NewOptions(v *viper.Viper) Options {
	o := Options{
		Admin:        v.GetString("upstreams.admin"),
		User:         v.GetString("upstreams.user"),
		Content:      v.GetString("upstreams.content"),
		Comment:      v.GetString("upstreams.comment"),
		Reaction:     v.GetString("upstreams.reaction"),
		Search:       v.GetString("upstreams.search"),
		Feed:         v.GetString("upstreams.feed"),
		Credit:       v.GetString("upstreams.credit"),
		Mall:         v.GetString("upstreams.mall"),
		Notification: v.GetString("upstreams.notification"),
	}
	o.applyDefaults()
	return o
}

func (o *Options) applyDefaults() {
	o.Admin = serviceNameOrDefault(o.Admin, "bbs-admin-service")
	o.User = serviceNameOrDefault(o.User, "bbs-user-service")
	o.Content = serviceNameOrDefault(o.Content, "bbs-content-service")
	o.Comment = serviceNameOrDefault(o.Comment, "bbs-comment-service")
	o.Reaction = serviceNameOrDefault(o.Reaction, "bbs-reaction-service")
	o.Search = serviceNameOrDefault(o.Search, "bbs-search-service")
	o.Feed = serviceNameOrDefault(o.Feed, "bbs-feed-service")
	o.Credit = serviceNameOrDefault(o.Credit, "bbs-credit-service")
	o.Mall = serviceNameOrDefault(o.Mall, "bbs-mall-service")
	o.Notification = serviceNameOrDefault(o.Notification, "bbs-notification-service")
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
	default:
		return strings.TrimSpace(value)
	}
}
