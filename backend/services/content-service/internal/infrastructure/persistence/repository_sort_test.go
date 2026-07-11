package persistence

import "testing"

func TestArticleListOrder(t *testing.T) {
	tests := []struct {
		sort string
		want string
	}{
		{sort: "", want: "published_at DESC NULLS LAST, id DESC"},
		{sort: "HOT", want: "view_count DESC, published_at DESC NULLS LAST, id DESC"},
		{sort: "active", want: "updated_at DESC, published_at DESC NULLS LAST, id DESC"},
		{sort: "unknown", want: "published_at DESC NULLS LAST, id DESC"},
	}
	for _, tt := range tests {
		if got := articleListOrder(tt.sort); got != tt.want {
			t.Fatalf("articleListOrder(%q) = %q, want %q", tt.sort, got, tt.want)
		}
	}
}

func TestTopicListOrder(t *testing.T) {
	tests := []struct {
		sort string
		want string
	}{
		{sort: "", want: "published_at DESC NULLS LAST, id DESC"},
		{sort: "hot", want: "view_count DESC, updated_at DESC, published_at DESC NULLS LAST, id DESC"},
		{sort: "recent-replies", want: "updated_at DESC, published_at DESC NULLS LAST, id DESC"},
		{sort: "unknown", want: "published_at DESC NULLS LAST, id DESC"},
	}
	for _, tt := range tests {
		if got := topicListOrder(tt.sort); got != tt.want {
			t.Fatalf("topicListOrder(%q) = %q, want %q", tt.sort, got, tt.want)
		}
	}
}
