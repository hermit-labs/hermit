package service

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"path"
	"sort"
	"strings"
	"time"

	"hermit/internal/store"
)

func buildZipArchive(files []PublishFileInput) ([]byte, []map[string]any, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	descriptors := make([]map[string]any, 0, len(files))

	for _, f := range files {
		safePath := sanitizeArchivePath(f.Path)
		if safePath == "" {
			_ = zw.Close()
			return nil, nil, fmt.Errorf("%w: invalid file path %q", ErrInvalidInput, f.Path)
		}

		descriptors = append(descriptors, buildFileDescriptor(safePath, f.ContentType, f.Bytes))

		header := &zip.FileHeader{
			Name:   safePath,
			Method: zip.Deflate,
		}
		header.SetModTime(time.Unix(0, 0).UTC())
		writer, err := zw.CreateHeader(header)
		if err != nil {
			_ = zw.Close()
			return nil, nil, err
		}
		if _, err := writer.Write(f.Bytes); err != nil {
			_ = zw.Close()
			return nil, nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), descriptors, nil
}

func describeZipArchiveFiles(r io.ReaderAt, size int64) ([]map[string]any, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("%w: artifact is not a zip", ErrInvalidInput)
	}

	descriptors := make([]map[string]any, 0, len(zr.File))
	for _, zf := range zr.File {
		safePath := sanitizeArchivePath(zf.Name)
		if safePath == "" || strings.HasSuffix(safePath, "/") || strings.HasSuffix(zf.Name, "/") {
			continue
		}

		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}

		descriptors = append(descriptors, buildFileDescriptor(safePath, mime.TypeByExtension(path.Ext(safePath)), data))
	}

	sort.SliceStable(descriptors, func(i, j int) bool {
		return descriptors[i]["path"].(string) < descriptors[j]["path"].(string)
	})
	return descriptors, nil
}

func buildFileDescriptor(path string, contentType string, body []byte) map[string]any {
	h := sha256.Sum256(body)
	desc := map[string]any{
		"path":   path,
		"size":   len(body),
		"sha256": hex.EncodeToString(h[:]),
	}
	contentType = strings.TrimSpace(contentType)
	if contentType != "" {
		desc["contentType"] = contentType
	}
	return desc
}

func hasSkillManifest(files []PublishFileInput) bool {
	for _, f := range files {
		lower := strings.ToLower(sanitizeArchivePath(f.Path))
		if lower == "skill.md" || lower == "skills.md" {
			return true
		}
	}
	return false
}

func sanitizeArchivePath(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	p = strings.TrimPrefix(p, "/")
	p = path.Clean(p)
	if p == "." || p == "" || p == "/" {
		return ""
	}
	if strings.HasPrefix(p, "../") || strings.Contains(p, "/../") || strings.HasPrefix(p, "..") {
		return ""
	}
	return p
}

func roleAllows(currentRole, requiredRole string) bool {
	if requiredRole == store.RoleRead {
		return currentRole == store.RoleRead || currentRole == store.RolePush || currentRole == store.RoleAdmin
	}
	if requiredRole == store.RolePush {
		return currentRole == store.RolePush || currentRole == store.RoleAdmin
	}
	if requiredRole == store.RoleAdmin {
		return currentRole == store.RoleAdmin
	}
	return false
}

func generateToken(lengthBytes int) (string, error) {
	if lengthBytes <= 0 {
		lengthBytes = 32
	}
	buf := make([]byte, lengthBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func normalizeSlug(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return ""
	}
	if strings.Contains(slug, "/") || strings.Contains(slug, "\\") || strings.Contains(slug, "..") {
		return ""
	}
	return slug
}

func sortSkillItems(items []store.SkillListItem, sortBy string) {
	sort.SliceStable(items, func(i, j int) bool {
		a := items[i]
		b := items[j]
		switch sortBy {
		case "downloads":
			if a.Downloads != b.Downloads {
				return a.Downloads > b.Downloads
			}
		case "stars":
			if a.Stars != b.Stars {
				return a.Stars > b.Stars
			}
		case "installsCurrent":
			if a.InstallsCurrent != b.InstallsCurrent {
				return a.InstallsCurrent > b.InstallsCurrent
			}
		case "installsAllTime":
			if a.InstallsAllTime != b.InstallsAllTime {
				return a.InstallsAllTime > b.InstallsAllTime
			}
		case "trending":
			if a.Downloads != b.Downloads {
				return a.Downloads > b.Downloads
			}
		default:
		}
		if a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.Slug < b.Slug
		}
		return a.UpdatedAt.After(b.UpdatedAt)
	})
}
