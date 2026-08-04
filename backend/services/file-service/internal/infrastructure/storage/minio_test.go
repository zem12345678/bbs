package storage

import (
	"testing"

	"github.com/spf13/viper"
)

func TestNewMinIODeleterAllowsUnconfiguredLocalCredentials(t *testing.T) {
	v := viper.New()
	v.Set("storage.endpoint", "http://127.0.0.1:9000")
	v.Set("storage.bucket", "bbs-local")

	deleter, err := NewMinIODeleter(v)
	if err != nil {
		t.Fatalf("NewMinIODeleter() error = %v", err)
	}
	if deleter != nil {
		t.Fatal("NewMinIODeleter() returned a deleter without credentials")
	}
}
