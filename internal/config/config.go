package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration for the API server.
type Config struct {
	ListenAddr           string
	DatabaseURL          string
	StorageRoot          string
	AdminToken           string
	CORSAllowedOrigins   []string
	DefaultHostedRepo    string
	DefaultGroupRepo     string
	DefaultProxyRepo     string
	ProxyUpstreamURLs    []string
	BootstrapDefaults    bool
	ProxyTimeout         time.Duration
	ProxyNegativeTTL     time.Duration
	ProxySyncEnabled     bool
	ProxySyncInterval    time.Duration
	ProxySyncDelay       time.Duration
	ProxySyncPageSize    int
	ProxySyncConcurrency int
	MaxUploadBytes       int64
	HTTPReadTimeout      time.Duration
	HTTPWriteTimeout     time.Duration
	HTTPIdleTimeout      time.Duration

	// LDAP authentication
	LDAPEnabled      bool
	LDAPURL          string
	LDAPBaseDN       string
	LDAPBindDN       string
	LDAPBindPassword string
	LDAPUserFilter   string
	LDAPUserAttr     string
	LDAPDisplayAttr  string
	LDAPStartTLS     bool
	LDAPSkipVerify   bool
	LDAPAdminGroups  []string

	// Initial admin user (created on bootstrap)
	AdminUsername string
	AdminPassword string
}

func Load() (Config, error) {
	defaultCORSOrigins := []string{"http://localhost:5173", "http://127.0.0.1:5173"}
	cfg := Config{
		ListenAddr:           getenv("LISTEN_ADDR", ":8080"),
		DatabaseURL:          getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/hermit?sslmode=disable"),
		StorageRoot:          getenv("STORAGE_ROOT", "./data"),
		AdminToken:           getenv("ADMIN_TOKEN", "dev-admin-token"),
		DefaultHostedRepo:    getenv("DEFAULT_HOSTED_REPO", "hosted"),
		DefaultGroupRepo:     getenv("DEFAULT_GROUP_REPO", "group"),
		DefaultProxyRepo:     getenv("DEFAULT_PROXY_REPO", "proxy"),
		BootstrapDefaults:    getenvBool("BOOTSTRAP_DEFAULT_REPOS", true),
		ProxyTimeout:         getenvDuration("PROXY_TIMEOUT", 30*time.Second),
		ProxyNegativeTTL:     getenvDuration("PROXY_NEGATIVE_TTL", 5*time.Minute),
		ProxySyncEnabled:     getenvBool("PROXY_SYNC_ENABLED", false),
		ProxySyncInterval:    getenvDuration("PROXY_SYNC_INTERVAL", 30*time.Minute),
		ProxySyncDelay:       getenvDuration("PROXY_SYNC_DELAY", 10*time.Second),
		ProxySyncPageSize:    getenvInt("PROXY_SYNC_PAGE_SIZE", 100),
		ProxySyncConcurrency: getenvInt("PROXY_SYNC_CONCURRENCY", 4),
		MaxUploadBytes:       getenvInt64("MAX_UPLOAD_BYTES", 128*1024*1024),
		HTTPReadTimeout:      getenvDuration("HTTP_READ_TIMEOUT", 15*time.Second),
		HTTPWriteTimeout:     getenvDuration("HTTP_WRITE_TIMEOUT", 60*time.Second),
		HTTPIdleTimeout:      getenvDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
	}
	cfg.CORSAllowedOrigins = parseList(getenv("CORS_ALLOWED_ORIGINS", strings.Join(defaultCORSOrigins, ",")))
	if len(cfg.CORSAllowedOrigins) == 0 {
		cfg.CORSAllowedOrigins = defaultCORSOrigins
	}
	cfg.ProxyUpstreamURLs = parseUpstreamURLs(
		strings.TrimSpace(os.Getenv("PROXY_UPSTREAM_URLS")),
		strings.TrimSpace(os.Getenv("PROXY_UPSTREAM_URL")),
	)

	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return Config{}, fmt.Errorf("DATABASE_URL cannot be empty")
	}
	if strings.TrimSpace(cfg.StorageRoot) == "" {
		return Config{}, fmt.Errorf("STORAGE_ROOT cannot be empty")
	}
	if strings.TrimSpace(cfg.AdminToken) == "" {
		return Config{}, fmt.Errorf("ADMIN_TOKEN cannot be empty")
	}
	if cfg.ProxySyncPageSize <= 0 {
		cfg.ProxySyncPageSize = 100
	}
	if cfg.ProxySyncConcurrency <= 0 {
		cfg.ProxySyncConcurrency = 1
	}
	if cfg.ProxySyncInterval < 0 {
		cfg.ProxySyncInterval = 0
	}
	if cfg.ProxySyncDelay < 0 {
		cfg.ProxySyncDelay = 0
	}

	// LDAP
	cfg.LDAPEnabled = getenvBool("LDAP_ENABLED", false)
	cfg.LDAPURL = getenv("LDAP_URL", "")
	cfg.LDAPBaseDN = getenv("LDAP_BASE_DN", "")
	cfg.LDAPBindDN = getenv("LDAP_BIND_DN", "")
	cfg.LDAPBindPassword = getenv("LDAP_BIND_PASSWORD", "")
	cfg.LDAPUserFilter = getenv("LDAP_USER_FILTER", "(uid={{.Username}})")
	cfg.LDAPUserAttr = getenv("LDAP_USER_ATTR", "uid")
	cfg.LDAPDisplayAttr = getenv("LDAP_DISPLAY_ATTR", "cn")
	cfg.LDAPStartTLS = getenvBool("LDAP_STARTTLS", false)
	cfg.LDAPSkipVerify = getenvBool("LDAP_SKIP_VERIFY", false)
	cfg.LDAPAdminGroups = parseList(getenv("LDAP_ADMIN_GROUPS", ""))

	// Initial admin user
	cfg.AdminUsername = getenv("ADMIN_USERNAME", "admin")
	cfg.AdminPassword = getenv("ADMIN_PASSWORD", "")

	return cfg, nil
}

func getenv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvInt64(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseUpstreamURLs(listRaw, singleRaw string) []string {
	var candidates []string
	if strings.TrimSpace(listRaw) != "" {
		candidates = append(candidates, parseList(listRaw)...)
	}
	if len(candidates) == 0 && strings.TrimSpace(singleRaw) != "" {
		candidates = append(candidates, strings.TrimSpace(singleRaw))
	}

	return dedupeNonEmpty(candidates)
}

func parseList(raw string) []string {
	replacer := strings.NewReplacer("\n", ",", ";", ",")
	normalized := replacer.Replace(raw)
	parts := strings.Split(normalized, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return dedupeNonEmpty(out)
}

func dedupeNonEmpty(candidates []string) []string {
	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		key := strings.ToLower(strings.TrimSpace(c))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, strings.TrimSpace(c))
	}
	return out
}
