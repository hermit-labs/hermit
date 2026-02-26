package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3BlobStore stores artifacts in an S3-compatible bucket (AWS S3, MinIO, etc.).
type S3BlobStore struct {
	client *s3.Client
	bucket string
	prefix string
}

var _ BlobStorage = (*S3BlobStore)(nil)

type S3Options struct {
	Client *s3.Client
	Bucket string
	Prefix string // optional key prefix, e.g. "blobs/"
}

func NewS3BlobStore(opts S3Options) *S3BlobStore {
	return &S3BlobStore{
		client: opts.Client,
		bucket: opts.Bucket,
		prefix: opts.Prefix,
	}
}

func (s *S3BlobStore) objectKey(key string) string {
	if s.prefix != "" {
		return s.prefix + key
	}
	return key
}

func (s *S3BlobStore) PutStream(ctx context.Context, r io.Reader) (digest string, size int64, key string, err error) {
	// Write to a temp file first to compute digest and get a seekable body.
	tmpFile, err := os.CreateTemp("", "s3-blob-*")
	if err != nil {
		return "", 0, "", fmt.Errorf("create tmp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
	}()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmpFile, h), r)
	if err != nil {
		return "", 0, "", fmt.Errorf("write tmp blob: %w", err)
	}

	sum := h.Sum(nil)
	hexDigest := hex.EncodeToString(sum)
	digest = "sha256:" + hexDigest
	size = n
	key = filepath.Join("sha256", hexDigest[:2], hexDigest)

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return "", 0, "", fmt.Errorf("seek tmp file: %w", err)
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.objectKey(key)),
		Body:          tmpFile,
		ContentLength: aws.Int64(n),
		ContentType:   aws.String("application/octet-stream"),
	})
	if err != nil {
		return "", 0, "", fmt.Errorf("s3 put: %w", err)
	}

	return digest, size, key, nil
}

func (s *S3BlobStore) Open(ctx context.Context, key string) (*BlobFile, error) {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get %q: %w", key, err)
	}
	defer resp.Body.Close()

	// Download to a temp file so callers get io.ReaderAt (needed for zip).
	tmpFile, err := os.CreateTemp("", "s3-read-*")
	if err != nil {
		return nil, fmt.Errorf("create tmp file: %w", err)
	}

	n, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("download s3 object: %w", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("seek tmp file: %w", err)
	}

	return NewBlobFile(tmpFile, n), nil
}
