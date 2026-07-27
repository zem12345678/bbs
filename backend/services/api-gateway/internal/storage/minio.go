package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/viper"
)

type MinIOStore struct {
	client *minio.Client
	bucket string
}

func NewMinIO(v *viper.Viper) (*MinIOStore, error) {
	endpoint := strings.TrimSpace(v.GetString("storage.endpoint"))
	if endpoint == "" {
		return nil, fmt.Errorf("storage.endpoint is required")
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("storage.endpoint must be an http or https URL")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return nil, fmt.Errorf("storage.endpoint must not contain a path")
	}
	bucket := strings.TrimSpace(v.GetString("storage.bucket"))
	accessKey := strings.TrimSpace(v.GetString("storage.accessKey"))
	secretKey := strings.TrimSpace(v.GetString("storage.secretKey"))
	if bucket == "" || accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("storage bucket and credentials are required")
	}
	client, err := minio.New(parsed.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: parsed.Scheme == "https",
	})
	if err != nil {
		return nil, err
	}
	return &MinIOStore{client: client, bucket: bucket}, nil
}

func (s *MinIOStore) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *MinIOStore) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, ObjectInfo{}, normalizeOpenError(err)
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return nil, ObjectInfo{}, normalizeOpenError(err)
	}
	return object, ObjectInfo{Size: info.Size, ContentType: info.ContentType}, nil
}

func normalizeOpenError(err error) error {
	if err == nil {
		return nil
	}
	response := minio.ToErrorResponse(err)
	switch response.Code {
	case "NoSuchKey", "NoSuchObject", "NoSuchBucket", "NotFound":
		return fmt.Errorf("%w: %v", ErrObjectNotFound, err)
	default:
		return err
	}
}

func (s *MinIOStore) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

// EnsureReady verifies that the configured bucket already exists. Buckets and
// their private policies are provisioned by infrastructure; the Gateway never
// creates or changes either at request time. This keeps the runtime credential
// free of bucket-policy and bucket-creation privileges while making startup
// fail before serving media operations with invalid credentials or a missing
// bucket.
func (s *MinIOStore) EnsureReady(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check storage bucket %q: %w", s.bucket, err)
	}
	if !exists {
		return fmt.Errorf("storage bucket %q does not exist", s.bucket)
	}
	return nil
}

var _ ObjectStore = (*MinIOStore)(nil)
