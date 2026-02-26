package storage

import (
	"context"
	"io"
	"os"
)

// BlobFile represents an opened blob that supports sequential read,
// random-access read (for zip parsing), and reports its size.
type BlobFile struct {
	file *os.File
	size int64
}

func NewBlobFile(f *os.File, size int64) *BlobFile {
	return &BlobFile{file: f, size: size}
}

func (b *BlobFile) Read(p []byte) (int, error)            { return b.file.Read(p) }
func (b *BlobFile) ReadAt(p []byte, off int64) (int, error) { return b.file.ReadAt(p, off) }
func (b *BlobFile) Close() error                           { return b.file.Close() }
func (b *BlobFile) Size() int64                            { return b.size }

// BlobStorage is the interface for blob storage backends.
// Both local-disk and S3-compatible stores implement this.
type BlobStorage interface {
	// PutStream writes data from r, returning the content-addressable digest,
	// byte count, and a backend-specific key (relPath) for later retrieval.
	PutStream(ctx context.Context, r io.Reader) (digest string, size int64, key string, err error)

	// Open retrieves a previously stored blob by its key.
	// The returned BlobFile must be closed by the caller.
	Open(ctx context.Context, key string) (*BlobFile, error)
}
