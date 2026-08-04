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

const deletePermissionProbeKey = ".bbs-health/file-service-delete-permission-check"

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

func (d *MinIODeleter) EnsureReady(ctx context.Context) error {
	if d == nil || d.client == nil || strings.TrimSpace(d.bucket) == "" {
		return fmt.Errorf("storage deleter is not configured")
	}
	exists, err := d.client.BucketExists(ctx, d.bucket)
	if err != nil {
		return fmt.Errorf("check storage bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("storage bucket %q does not exist", d.bucket)
	}
	if err := d.client.RemoveObject(ctx, d.bucket, deletePermissionProbeKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("verify storage delete permission: %w", err)
	}
	return nil
}
