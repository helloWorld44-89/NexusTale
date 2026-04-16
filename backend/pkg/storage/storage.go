package storage

// Storage wraps MinIO / S3 for binary assets:
// cover images, exported EPUB/Scrivener files, and wiki entity reference images.

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client wraps a MinIO connection and the target bucket name.
type Client struct {
	mc     *minio.Client
	bucket string
}

// Config mirrors the fields from internal/config.MinioConfig.
// Duplicated here to avoid an import cycle between pkg/storage ↔ internal/config.
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// New connects to MinIO and ensures the target bucket exists.
// Returns an error if the connection or bucket creation fails.
func New(cfg Config) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio connect: %w", err)
	}

	ctx := context.Background()
	exists, err := mc.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("minio bucket check: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("minio make bucket: %w", err)
		}
	}

	return &Client{mc: mc, bucket: cfg.Bucket}, nil
}

// PutObject uploads r to the given key. size should be the byte length of r,
// or -1 if unknown (MinIO will buffer in that case).
func (c *Client) PutObject(ctx context.Context, key, contentType string, r io.Reader, size int64) error {
	_, err := c.mc.PutObject(ctx, c.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("minio put %s: %w", key, err)
	}
	return nil
}

// PresignedGetURL returns a time-limited download URL for the given object key.
func (c *Client) PresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := c.mc.PresignedGetObject(ctx, c.bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("minio presign %s: %w", key, err)
	}
	return u.String(), nil
}

// DeleteObject removes the object at key. A missing key is not an error.
func (c *Client) DeleteObject(ctx context.Context, key string) error {
	err := c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("minio delete %s: %w", key, err)
	}
	return nil
}
