package admin

import "testing"

func TestIsProtectedSystemRoleKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "admin", key: "admin", want: true},
		{name: "superadmin", key: "superadmin", want: true},
		{name: "case and space", key: " Admin ", want: true},
		{name: "normal role", key: "moderator", want: false},
		{name: "empty", key: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsProtectedSystemRoleKey(tt.key); got != tt.want {
				t.Fatalf("IsProtectedSystemRoleKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
