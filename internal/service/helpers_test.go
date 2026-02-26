package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"hermit/internal/store"
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

func TestSanitizeArchivePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"SKILL.md", "SKILL.md"},
		{"docs/guide.txt", "docs/guide.txt"},
		{"/leading-slash.txt", "leading-slash.txt"},
		{"  spaces.txt  ", "spaces.txt"},
		{"back\\slash.txt", "back/slash.txt"},
		{"../escape", ""},
		{"foo/../../bar", ""},
		{"", ""},
		{".", ""},
		{"/", ""},
		{"normal/path/file.go", "normal/path/file.go"},
		{"./relative/path.txt", "relative/path.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := sanitizeArchivePath(tt.input)
			if got != tt.want {
				t.Fatalf("sanitizeArchivePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHasSkillManifest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		files []PublishFileInput
		want  bool
	}{
		{"has SKILL.md", []PublishFileInput{{Path: "SKILL.md"}}, true},
		{"has skills.md", []PublishFileInput{{Path: "skills.md"}}, true},
		{"case insensitive", []PublishFileInput{{Path: "Skill.MD"}}, true},
		{"no manifest", []PublishFileInput{{Path: "README.md"}}, false},
		{"empty", nil, false},
		{"manifest in subdir is not counted", []PublishFileInput{{Path: "docs/SKILL.md"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := hasSkillManifest(tt.files)
			if got != tt.want {
				t.Fatalf("hasSkillManifest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRoleAllows(t *testing.T) {
	t.Parallel()
	tests := []struct {
		current  string
		required string
		want     bool
	}{
		{store.RoleAdmin, store.RoleAdmin, true},
		{store.RoleAdmin, store.RolePush, true},
		{store.RoleAdmin, store.RoleRead, true},
		{store.RolePush, store.RoleAdmin, false},
		{store.RolePush, store.RolePush, true},
		{store.RolePush, store.RoleRead, true},
		{store.RoleRead, store.RoleAdmin, false},
		{store.RoleRead, store.RolePush, false},
		{store.RoleRead, store.RoleRead, true},
		{"unknown", store.RoleRead, false},
		{store.RoleAdmin, "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.current+"_requires_"+tt.required, func(t *testing.T) {
			t.Parallel()
			got := roleAllows(tt.current, tt.required)
			if got != tt.want {
				t.Fatalf("roleAllows(%q, %q) = %v, want %v", tt.current, tt.required, got, tt.want)
			}
		})
	}
}

func TestNormalizeSlug(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"  Hello  ", "hello"},
		{"UPPER-CASE", "upper-case"},
		{"", ""},
		{"  ", ""},
		{"has/slash", ""},
		{"has\\backslash", ""},
		{"has..dots", ""},
		{"valid-slug-123", "valid-slug-123"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := normalizeSlug(tt.input)
			if got != tt.want {
				t.Fatalf("normalizeSlug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildFileDescriptor(t *testing.T) {
	t.Parallel()
	body := []byte("hello world")
	desc := buildFileDescriptor("test.txt", "text/plain", body)

	if desc["path"] != "test.txt" {
		t.Fatalf("path = %v, want test.txt", desc["path"])
	}
	if desc["size"] != len(body) {
		t.Fatalf("size = %v, want %d", desc["size"], len(body))
	}
	if desc["sha256"] != shaHex(body) {
		t.Fatalf("sha256 = %v, want %s", desc["sha256"], shaHex(body))
	}
	if desc["contentType"] != "text/plain" {
		t.Fatalf("contentType = %v, want text/plain", desc["contentType"])
	}
}

func TestBuildFileDescriptor_EmptyContentType(t *testing.T) {
	t.Parallel()
	desc := buildFileDescriptor("file.bin", "", []byte{0x00})
	if _, ok := desc["contentType"]; ok {
		t.Fatal("contentType should be absent for empty content type")
	}
}

func TestSortSkillItems(t *testing.T) {
	t.Parallel()

	now := time.Now()
	items := []store.SkillListItem{
		{Skill: store.Skill{Slug: "b", Downloads: 10, UpdatedAt: now}},
		{Skill: store.Skill{Slug: "a", Downloads: 50, UpdatedAt: now.Add(-time.Hour)}},
		{Skill: store.Skill{Slug: "c", Downloads: 50, UpdatedAt: now}},
	}

	sortSkillItems(items, "downloads")

	if items[0].Slug != "c" {
		t.Fatalf("first = %q, want c (50 downloads, newer)", items[0].Slug)
	}
	if items[1].Slug != "a" {
		t.Fatalf("second = %q, want a (50 downloads, older)", items[1].Slug)
	}
	if items[2].Slug != "b" {
		t.Fatalf("third = %q, want b (10 downloads)", items[2].Slug)
	}
}

func TestSortSkillItems_DefaultSort(t *testing.T) {
	t.Parallel()

	now := time.Now()
	items := []store.SkillListItem{
		{Skill: store.Skill{Slug: "old", UpdatedAt: now.Add(-time.Hour)}},
		{Skill: store.Skill{Slug: "new", UpdatedAt: now}},
		{Skill: store.Skill{Slug: "same-time-b", UpdatedAt: now.Add(-time.Minute)}},
		{Skill: store.Skill{Slug: "same-time-a", UpdatedAt: now.Add(-time.Minute)}},
	}

	sortSkillItems(items, "")

	if items[0].Slug != "new" {
		t.Fatalf("first = %q, want new", items[0].Slug)
	}
	if items[1].Slug != "same-time-a" {
		t.Fatalf("second = %q, want same-time-a (alphabetical tiebreak)", items[1].Slug)
	}
	if items[2].Slug != "same-time-b" {
		t.Fatalf("third = %q, want same-time-b", items[2].Slug)
	}
}
