package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// BlobStore stores artifacts by sha256 digest on local disk.
type BlobStore struct {
	root string
}

func NewBlobStore(root string) (*BlobStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}
	return &BlobStore{root: root}, nil
}

func (b *BlobStore) PutStream(_ context.Context, r io.Reader) (digest string, size int64, relPath string, err error) {
	tmpDir := filepath.Join(b.root, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", 0, "", fmt.Errorf("create tmp dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(tmpDir, "blob-*")
	if err != nil {
		return "", 0, "", fmt.Errorf("create tmp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmpFile, h), r)
	if err != nil {
		return "", 0, "", fmt.Errorf("write blob: %w", err)
	}
	sum := h.Sum(nil)
	digest = "sha256:" + hex.EncodeToString(sum)
	size = n

	hexDigest := hex.EncodeToString(sum)
	relPath = filepath.Join("sha256", hexDigest[:2], hexDigest)
	absPath := filepath.Join(b.root, relPath)

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", 0, "", fmt.Errorf("create blob dir: %w", err)
	}
	if _, statErr := os.Stat(absPath); statErr == nil {
		_ = os.Remove(tmpName)
		return digest, size, relPath, nil
	}

	if err := tmpFile.Close(); err != nil {
		return "", 0, "", fmt.Errorf("close tmp file: %w", err)
	}

	if err := os.Rename(tmpName, absPath); err != nil {
		return "", 0, "", fmt.Errorf("move blob: %w", err)
	}
	return digest, size, relPath, nil
}

func (b *BlobStore) Open(relPath string) (*os.File, os.FileInfo, error) {
	absPath := filepath.Join(b.root, relPath)
	f, err := os.Open(absPath)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	return f, info, nil
}
