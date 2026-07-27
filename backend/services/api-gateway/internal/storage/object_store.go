package storage

import (
	"context"
	"errors"
	"io"
)

// ErrObjectNotFound lets callers distinguish a missing object from a storage
// outage without depending on a concrete object-store implementation.
var ErrObjectNotFound = errors.New("object not found")

type ObjectInfo struct {
	Size        int64
	ContentType string
}

type ObjectStore interface {
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
	Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key string) error
}
