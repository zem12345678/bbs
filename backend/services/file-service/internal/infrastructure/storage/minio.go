package storage

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/viper"
)

type MinIODeleter struct {
	client *minio.Client
	bucket string
}

func NewMinIODeleter(v *viper.Viper) (*MinIODeleter, error) {
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
	if accessKey == "" && secretKey == "" {
		return nil, nil
	}
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
	return &MinIODeleter{client: client, bucket: bucket}, nil
}

func (d *MinIODeleter) Delete(ctx context.Context, objectKey string) error {
	return d.client.RemoveObject(ctx, d.bucket, objectKey, minio.RemoveObjectOptions{})
}
