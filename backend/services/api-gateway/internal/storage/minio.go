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
	if err := s.ensureBucket(ctx); err != nil {
		return err
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *MinIOStore) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return nil, ObjectInfo{}, err
	}
	return object, ObjectInfo{Size: info.Size, ContentType: info.ContentType}, nil
}

func (s *MinIOStore) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *MinIOStore) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		if minio.ToErrorResponse(err).Code != "BucketAlreadyOwnedByYou" {
			return err
		}
	}
	return nil
}

var _ ObjectStore = (*MinIOStore)(nil)
