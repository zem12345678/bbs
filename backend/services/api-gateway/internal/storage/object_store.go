package storage

import (
	"context"
	"io"
)

type ObjectInfo struct {
	Size        int64
	ContentType string
}

type ObjectStore interface {
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
	Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key string) error
}
