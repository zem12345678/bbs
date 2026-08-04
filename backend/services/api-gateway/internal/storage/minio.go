package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/viper"
)

type MinIOStore struct {
	client *minio.Client
	bucket string
}

const (
	readinessProbeObjectPrefix   = ".bbs-health/api-gateway-storage-readiness/"
	readinessProbeCleanupTimeout = 5 * time.Second
)

type bucketReadinessProbe interface {
	BucketExists(ctx context.Context, bucket string) (bool, error)
	WriteObject(ctx context.Context, bucket, key string, content []byte) error
	ReadObject(ctx context.Context, bucket, key string) ([]byte, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	ObjectExists(ctx context.Context, bucket, key string) (bool, error)
}

type minioBucketReadinessProbe struct {
	client *minio.Client
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
	if isMinIOObjectNotFound(err) {
		return fmt.Errorf("%w: %v", ErrObjectNotFound, err)
	}
	return err
}

func (s *MinIOStore) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

// EnsureReady verifies the bucket and every object permission used at runtime.
// The probe object is unique to this process attempt and is removed before the
// Gateway starts serving requests.
func (s *MinIOStore) EnsureReady(ctx context.Context) error {
	if s == nil || s.client == nil || strings.TrimSpace(s.bucket) == "" {
		return fmt.Errorf("storage is not configured")
	}
	return ensureBucketReady(ctx, minioBucketReadinessProbe{client: s.client}, s.bucket)
}

func ensureBucketReady(ctx context.Context, probe bucketReadinessProbe, bucket string) (resultErr error) {
	exists, err := probe.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check storage bucket %q: %w", bucket, err)
	}
	if !exists {
		return fmt.Errorf("storage bucket %q does not exist", bucket)
	}

	objectKey := readinessProbeObjectPrefix + uuid.NewString()
	expected := []byte("bbs-api-gateway-storage-readiness:" + objectKey)
	cleanupRequired := true
	defer func() {
		if !cleanupRequired {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), readinessProbeCleanupTimeout)
		defer cleanupCancel()
		if err := probe.DeleteObject(cleanupCtx, bucket, objectKey); err != nil {
			cleanupErr := fmt.Errorf("clean up storage readiness probe %q: %w", objectKey, err)
			if resultErr == nil {
				resultErr = cleanupErr
			} else {
				resultErr = errors.Join(resultErr, cleanupErr)
			}
		}
	}()

	if err := probe.WriteObject(ctx, bucket, objectKey, expected); err != nil {
		return fmt.Errorf("write storage readiness probe %q: %w", objectKey, err)
	}
	actual, err := probe.ReadObject(ctx, bucket, objectKey)
	if err != nil {
		return fmt.Errorf("read storage readiness probe %q: %w", objectKey, err)
	}
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("storage readiness probe %q content mismatch", objectKey)
	}
	if err := probe.DeleteObject(ctx, bucket, objectKey); err != nil {
		return fmt.Errorf("delete storage readiness probe %q: %w", objectKey, err)
	}
	exists, err = probe.ObjectExists(ctx, bucket, objectKey)
	if err != nil {
		return fmt.Errorf("confirm storage readiness probe %q deletion: %w", objectKey, err)
	}
	if exists {
		return fmt.Errorf("storage readiness probe %q still exists after deletion", objectKey)
	}
	cleanupRequired = false
	return nil
}

func (p minioBucketReadinessProbe) BucketExists(ctx context.Context, bucket string) (bool, error) {
	return p.client.BucketExists(ctx, bucket)
}

func (p minioBucketReadinessProbe) WriteObject(ctx context.Context, bucket, key string, content []byte) error {
	_, err := p.client.PutObject(ctx, bucket, key, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	return err
}

func (p minioBucketReadinessProbe) ReadObject(ctx context.Context, bucket, key string) ([]byte, error) {
	object, err := p.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(object)
	closeErr := object.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return content, nil
}

func (p minioBucketReadinessProbe) DeleteObject(ctx context.Context, bucket, key string) error {
	return p.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

func (p minioBucketReadinessProbe) ObjectExists(ctx context.Context, bucket, key string) (bool, error) {
	if _, err := p.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{}); err != nil {
		if isMinIOObjectNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isMinIOObjectNotFound(err error) bool {
	switch minio.ToErrorResponse(err).Code {
	case "NoSuchKey", "NoSuchObject", "NotFound":
		return true
	default:
		return false
	}
}

var _ ObjectStore = (*MinIOStore)(nil)
