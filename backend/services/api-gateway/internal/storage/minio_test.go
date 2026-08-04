package storage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
)

func TestEnsureBucketReadyVerifiesWriteReadAndDeleteWithUniqueObjects(t *testing.T) {
	probe := newFakeBucketReadinessProbe()

	require.NoError(t, ensureBucketReady(t.Context(), probe, "bbs-test"))
	require.NoError(t, ensureBucketReady(t.Context(), probe, "bbs-test"))

	require.Len(t, probe.writeKeys, 2)
	require.NotEqual(t, probe.writeKeys[0], probe.writeKeys[1])
	for _, key := range probe.writeKeys {
		require.True(t, strings.HasPrefix(key, readinessProbeObjectPrefix), key)
	}
	require.Empty(t, probe.objects)
	require.Equal(t, []string{
		"bucket",
		"write:" + probe.writeKeys[0],
		"read:" + probe.writeKeys[0],
		"delete:" + probe.writeKeys[0],
		"exists:" + probe.writeKeys[0],
		"bucket",
		"write:" + probe.writeKeys[1],
		"read:" + probe.writeKeys[1],
		"delete:" + probe.writeKeys[1],
		"exists:" + probe.writeKeys[1],
	}, probe.calls)
}

func TestEnsureBucketReadyFailsClosedAndCleansProbe(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*fakeBucketReadinessProbe)
		wantError string
	}{
		{
			name: "bucket check fails",
			configure: func(probe *fakeBucketReadinessProbe) {
				probe.bucketErr = errors.New("list denied")
			},
			wantError: "check storage bucket",
		},
		{
			name: "bucket missing",
			configure: func(probe *fakeBucketReadinessProbe) {
				probe.bucketPresent = false
			},
			wantError: "does not exist",
		},
		{
			name: "write fails after partial object creation",
			configure: func(probe *fakeBucketReadinessProbe) {
				probe.writeErr = errors.New("write interrupted")
				probe.storeOnWriteError = true
			},
			wantError: "write storage readiness probe",
		},
		{
			name: "read permission denied",
			configure: func(probe *fakeBucketReadinessProbe) {
				probe.readErr = errors.New("read denied")
			},
			wantError: "read storage readiness probe",
		},
		{
			name: "read content mismatch",
			configure: func(probe *fakeBucketReadinessProbe) {
				probe.hasReadOverride = true
				probe.readOverride = []byte("wrong content")
			},
			wantError: "content mismatch",
		},
		{
			name: "delete permission denied",
			configure: func(probe *fakeBucketReadinessProbe) {
				probe.deleteErrs = []error{errors.New("delete denied")}
			},
			wantError: "delete storage readiness probe",
		},
		{
			name: "deletion confirmation fails",
			configure: func(probe *fakeBucketReadinessProbe) {
				probe.existsErrs = []error{errors.New("stat denied")}
			},
			wantError: "confirm storage readiness probe",
		},
		{
			name: "object remains after delete",
			configure: func(probe *fakeBucketReadinessProbe) {
				probe.keepOnDelete = map[int]bool{0: true}
			},
			wantError: "still exists after deletion",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			probe := newFakeBucketReadinessProbe()
			test.configure(probe)

			err := ensureBucketReady(t.Context(), probe, "bbs-test")

			require.ErrorContains(t, err, test.wantError)
			require.Empty(t, probe.objects)
		})
	}
}

func TestEnsureBucketReadyReportsCleanupFailure(t *testing.T) {
	probe := newFakeBucketReadinessProbe()
	probe.writeErr = errors.New("write interrupted")
	probe.storeOnWriteError = true
	probe.deleteErrs = []error{errors.New("cleanup denied")}

	err := ensureBucketReady(t.Context(), probe, "bbs-test")

	require.ErrorContains(t, err, "write interrupted")
	require.ErrorContains(t, err, "clean up storage readiness probe")
	require.ErrorContains(t, err, "cleanup denied")
	require.Len(t, probe.objects, 1)
	require.Equal(t, 1, probe.deleteCalls)
}

func TestIsMinIOObjectNotFound(t *testing.T) {
	require.True(t, isMinIOObjectNotFound(minio.ErrorResponse{Code: "NoSuchKey"}))
	require.True(t, isMinIOObjectNotFound(minio.ErrorResponse{Code: "NoSuchObject"}))
	require.False(t, isMinIOObjectNotFound(minio.ErrorResponse{Code: "NoSuchBucket"}))
	require.False(t, isMinIOObjectNotFound(minio.ErrorResponse{Code: "AccessDenied"}))
}

type fakeBucketReadinessProbe struct {
	bucketPresent     bool
	bucketErr         error
	objects           map[string][]byte
	writeErr          error
	storeOnWriteError bool
	readErr           error
	hasReadOverride   bool
	readOverride      []byte
	deleteErrs        []error
	existsErrs        []error
	keepOnDelete      map[int]bool
	deleteCalls       int
	existsCalls       int
	writeKeys         []string
	calls             []string
}

func newFakeBucketReadinessProbe() *fakeBucketReadinessProbe {
	return &fakeBucketReadinessProbe{
		bucketPresent: true,
		objects:       make(map[string][]byte),
		keepOnDelete:  make(map[int]bool),
	}
}

func (p *fakeBucketReadinessProbe) BucketExists(context.Context, string) (bool, error) {
	p.calls = append(p.calls, "bucket")
	return p.bucketPresent, p.bucketErr
}

func (p *fakeBucketReadinessProbe) WriteObject(_ context.Context, _, key string, content []byte) error {
	p.calls = append(p.calls, "write:"+key)
	p.writeKeys = append(p.writeKeys, key)
	if p.writeErr == nil || p.storeOnWriteError {
		p.objects[key] = append([]byte(nil), content...)
	}
	return p.writeErr
}

func (p *fakeBucketReadinessProbe) ReadObject(_ context.Context, _, key string) ([]byte, error) {
	p.calls = append(p.calls, "read:"+key)
	if p.readErr != nil {
		return nil, p.readErr
	}
	if p.hasReadOverride {
		return append([]byte(nil), p.readOverride...), nil
	}
	return append([]byte(nil), p.objects[key]...), nil
}

func (p *fakeBucketReadinessProbe) DeleteObject(_ context.Context, _, key string) error {
	p.calls = append(p.calls, "delete:"+key)
	call := p.deleteCalls
	p.deleteCalls++
	if call < len(p.deleteErrs) && p.deleteErrs[call] != nil {
		return p.deleteErrs[call]
	}
	if !p.keepOnDelete[call] {
		delete(p.objects, key)
	}
	return nil
}

func (p *fakeBucketReadinessProbe) ObjectExists(_ context.Context, _, key string) (bool, error) {
	p.calls = append(p.calls, "exists:"+key)
	call := p.existsCalls
	p.existsCalls++
	if call < len(p.existsErrs) && p.existsErrs[call] != nil {
		return false, p.existsErrs[call]
	}
	_, exists := p.objects[key]
	return exists, nil
}
