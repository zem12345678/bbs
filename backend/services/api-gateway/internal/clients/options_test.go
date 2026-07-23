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
	v.Set("upstreams.chat", " custom-chat ")

	o := NewOptions(v)

	if o.Admin != "custom-admin" {
		t.Fatalf("admin = %q", o.Admin)
	}
	if o.Chat != "custom-chat" {
		t.Fatalf("chat = %q", o.Chat)
	}
}
