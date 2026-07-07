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
		Notification: v.GetString("upstreams.notification"),
	}
	o.applyDefaults()
	return o
}

func (o *Options) applyDefaults() {
	if strings.TrimSpace(o.Admin) == "" {
		o.Admin = "admin-service"
	}
	if strings.TrimSpace(o.User) == "" {
		o.User = "user-service"
	}
	if strings.TrimSpace(o.Content) == "" {
		o.Content = "content-service"
	}
	if strings.TrimSpace(o.Comment) == "" {
		o.Comment = "comment-service"
	}
	if strings.TrimSpace(o.Reaction) == "" {
		o.Reaction = "reaction-service"
	}
	if strings.TrimSpace(o.Search) == "" {
		o.Search = "search-service"
	}
	if strings.TrimSpace(o.Feed) == "" {
		o.Feed = "feed-service"
	}
	if strings.TrimSpace(o.Credit) == "" {
		o.Credit = "credit-service"
	}
	if strings.TrimSpace(o.Notification) == "" {
		o.Notification = "notification-service"
	}
}
