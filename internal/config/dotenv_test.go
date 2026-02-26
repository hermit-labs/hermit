package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDotEnv_FileNotFoundIsIgnored(t *testing.T) {
	t.Parallel()
	if err := LoadDotEnv(filepath.Join(t.TempDir(), ".env")); err != nil {
		t.Fatalf("LoadDotEnv() error = %v", err)
	}
}

func TestLoadDotEnv_LoadsValuesAndRespectsExistingEnv(t *testing.T) {
	t.Setenv("DOTENV_KEEP", "from-process")

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := strings.Join([]string{
		"# comment",
		"DOTENV_A=1",
		"DOTENV_B=hello world",
		"DOTENV_KEEP=from-file",
		`DOTENV_QUOTED="a b c"`,
		"export DOTENV_EXPORTED=yes",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv() error = %v", err)
	}

	if got := os.Getenv("DOTENV_A"); got != "1" {
		t.Fatalf("DOTENV_A = %q, want %q", got, "1")
	}
	if got := os.Getenv("DOTENV_B"); got != "hello world" {
		t.Fatalf("DOTENV_B = %q, want %q", got, "hello world")
	}
	if got := os.Getenv("DOTENV_QUOTED"); got != "a b c" {
		t.Fatalf("DOTENV_QUOTED = %q, want %q", got, "a b c")
	}
	if got := os.Getenv("DOTENV_EXPORTED"); got != "yes" {
		t.Fatalf("DOTENV_EXPORTED = %q, want %q", got, "yes")
	}
	if got := os.Getenv("DOTENV_KEEP"); got != "from-process" {
		t.Fatalf("DOTENV_KEEP = %q, want %q", got, "from-process")
	}
}

func TestLoadDotEnv_InvalidLineReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(`DOTENV_BAD="unterminated`), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	if err := LoadDotEnv(path); err == nil {
		t.Fatalf("LoadDotEnv() error = nil, want non-nil")
	}
}
