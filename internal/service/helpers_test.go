package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

func TestDescribeZipArchiveFiles(t *testing.T) {
	t.Parallel()

	files := []PublishFileInput{
		{
			Path:        "SKILL.md",
			ContentType: "text/markdown",
			Bytes:       []byte("# Demo\nhello"),
		},
		{
			Path:  "docs/guide.txt",
			Bytes: []byte("guide"),
		},
	}

	archiveBytes, _, err := buildZipArchive(files)
	if err != nil {
		t.Fatalf("buildZipArchive() error = %v", err)
	}

	descriptors, err := describeZipArchiveFiles(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		t.Fatalf("describeZipArchiveFiles() error = %v", err)
	}
	if len(descriptors) != 2 {
		t.Fatalf("descriptors len = %d, want 2", len(descriptors))
	}

	first := descriptors[0]
	if first["path"] != "SKILL.md" {
		t.Fatalf("first path = %#v, want SKILL.md", first["path"])
	}
	if first["sha256"] != shaHex([]byte("# Demo\nhello")) {
		t.Fatalf("first sha256 = %#v, want %q", first["sha256"], shaHex([]byte("# Demo\nhello")))
	}
	if _, ok := first["content"]; ok {
		t.Fatalf("first descriptor should not include content")
	}

	second := descriptors[1]
	if second["path"] != "docs/guide.txt" {
		t.Fatalf("second path = %#v, want docs/guide.txt", second["path"])
	}
	if second["sha256"] != shaHex([]byte("guide")) {
		t.Fatalf("second sha256 = %#v, want %q", second["sha256"], shaHex([]byte("guide")))
	}
	if _, ok := second["content"]; ok {
		t.Fatalf("second descriptor should not include content")
	}
}

func TestDescribeZipArchiveFiles_InvalidZip(t *testing.T) {
	t.Parallel()

	_, err := describeZipArchiveFiles(bytes.NewReader([]byte("not-a-zip")), int64(len("not-a-zip")))
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestDescribeZipArchiveFiles_BinaryFile_NoInlineContent(t *testing.T) {
	t.Parallel()

	files := []PublishFileInput{
		{
			Path:  "bin/blob.bin",
			Bytes: []byte{0x00, 0x01, 0xFE, 0xFF},
		},
	}

	archiveBytes, _, err := buildZipArchive(files)
	if err != nil {
		t.Fatalf("buildZipArchive() error = %v", err)
	}

	descriptors, err := describeZipArchiveFiles(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		t.Fatalf("describeZipArchiveFiles() error = %v", err)
	}
	if len(descriptors) != 1 {
		t.Fatalf("descriptors len = %d, want 1", len(descriptors))
	}

	got := descriptors[0]
	if _, ok := got["content"]; ok {
		t.Fatalf("descriptor should not include content")
	}
}

func shaHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
