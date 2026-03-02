package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := `server:
  listen: ":8080"
auth:
  downstream_api_key: "fixed-key"
oauth:
  client_id: "client-id"
  device_authorization_endpoint: "https://oauth.example.com/device"
  token_endpoint: "https://oauth.example.com/token"
  scopes:
    - "openid"
upstream:
  base_url: "https://api.example.com"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}

	if cfg.Server.Listen != ":8080" {
		t.Fatalf("unexpected listen: %q", cfg.Server.Listen)
	}

	if cfg.Auth.DownstreamAPIKey != "fixed-key" {
		t.Fatalf("unexpected downstream key: %q", cfg.Auth.DownstreamAPIKey)
	}

	if cfg.Upstream.ModelsPath != "/v1/models" {
		t.Fatalf("expected default models path, got %q", cfg.Upstream.ModelsPath)
	}

	if cfg.Upstream.ChatCompletionsPath != "/v1/chat/completions" {
		t.Fatalf("expected default chat path, got %q", cfg.Upstream.ChatCompletionsPath)
	}
}

func TestLoad_MissingRequiredField(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := `server:
  listen: ":8080"
auth:
  downstream_api_key: ""
oauth:
  client_id: "client-id"
  device_authorization_endpoint: "https://oauth.example.com/device"
  token_endpoint: "https://oauth.example.com/token"
upstream:
  base_url: "https://api.example.com"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	if !strings.Contains(err.Error(), "auth.downstream_api_key") {
		t.Fatalf("expected downstream api key validation error, got: %v", err)
	}
}

func TestLoad_MinimalConfigAppliesOAuthDefaults(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := `auth:
  downstream_api_key: "fixed-key"
upstream:
  base_url: "https://api.example.com"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected minimal config to be valid, got: %v", err)
	}

	if cfg.OAuth.ClientID == "" {
		t.Fatal("expected default oauth.client_id")
	}

	if cfg.OAuth.TokenEndpoint == "" {
		t.Fatal("expected default oauth.token_endpoint")
	}

	if cfg.OAuth.AuthorizeEndpoint == "" {
		t.Fatal("expected default oauth.authorize_endpoint")
	}

	if cfg.OAuth.RedirectHost != "localhost" {
		t.Fatalf("expected default oauth.redirect_host localhost, got %q", cfg.OAuth.RedirectHost)
	}

	if cfg.Upstream.Mode != "codex_oauth" {
		t.Fatalf("expected default upstream.mode codex_oauth, got %q", cfg.Upstream.Mode)
	}

	if cfg.Upstream.CodexBaseURL != "https://chatgpt.com" {
		t.Fatalf("expected default upstream.codex_base_url, got %q", cfg.Upstream.CodexBaseURL)
	}
}

func TestLoad_NormalizesLoopbackRedirectHostToLocalhost(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := `auth:
  downstream_api_key: "fixed-key"
oauth:
  redirect_host: "127.0.0.1"
upstream:
  base_url: "https://api.example.com"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected config to load, got: %v", err)
	}

	if cfg.OAuth.RedirectHost != "localhost" {
		t.Fatalf("expected redirect_host localhost, got %q", cfg.OAuth.RedirectHost)
	}
}

func TestLoad_OpenAIApiModeRequiresBaseURL(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := `auth:
  downstream_api_key: "fixed-key"
upstream:
  mode: "openai_api"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "upstream.base_url") {
		t.Fatalf("expected upstream.base_url error, got: %v", err)
	}
}

func TestLoad_LoggingDefaults(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := `auth:
  downstream_api_key: "fixed-key"
upstream:
  base_url: "https://api.example.com"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected config to load, got: %v", err)
	}

	if cfg.Logging.Level != "info" {
		t.Fatalf("expected default logging.level info, got %q", cfg.Logging.Level)
	}

	if cfg.Logging.Format != "text" {
		t.Fatalf("expected default logging.format text, got %q", cfg.Logging.Format)
	}
}

func TestLoad_InvalidLoggingLevel(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := `auth:
  downstream_api_key: "fixed-key"
logging:
  level: "verbose"
upstream:
  base_url: "https://api.example.com"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "logging.level") {
		t.Fatalf("expected logging.level validation error, got: %v", err)
	}
}

func TestLoad_InvalidLoggingFormat(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := `auth:
  downstream_api_key: "fixed-key"
logging:
  format: "pretty"
upstream:
  base_url: "https://api.example.com"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "logging.format") {
		t.Fatalf("expected logging.format validation error, got: %v", err)
	}
}

func TestLoad_LoggingOutputDefaults(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := `auth:
  downstream_api_key: "fixed-key"
upstream:
  base_url: "https://api.example.com"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected config to load, got: %v", err)
	}

	if cfg.Logging.Output != "stdout" {
		t.Fatalf("expected default logging.output stdout, got %q", cfg.Logging.Output)
	}

	if cfg.Logging.Color != "auto" {
		t.Fatalf("expected default logging.color auto, got %q", cfg.Logging.Color)
	}

	if cfg.Logging.File.Name != "codex-gateway.log" {
		t.Fatalf("expected default logging.file.name codex-gateway.log, got %q", cfg.Logging.File.Name)
	}

	if cfg.Logging.File.MaxSizeMB != 100 {
		t.Fatalf("expected default logging.file.max_size_mb 100, got %d", cfg.Logging.File.MaxSizeMB)
	}

	if cfg.Logging.File.MaxBackups != 10 {
		t.Fatalf("expected default logging.file.max_backups 10, got %d", cfg.Logging.File.MaxBackups)
	}

	if cfg.Logging.File.MaxAgeDays != 7 {
		t.Fatalf("expected default logging.file.max_age_days 7, got %d", cfg.Logging.File.MaxAgeDays)
	}
}

func TestLoad_InvalidLoggingOutput(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := `auth:
  downstream_api_key: "fixed-key"
logging:
  output: "console"
upstream:
  base_url: "https://api.example.com"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "logging.output") {
		t.Fatalf("expected logging.output validation error, got: %v", err)
	}
}

func TestLoad_InvalidLoggingColor(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := `auth:
  downstream_api_key: "fixed-key"
logging:
  color: "on"
upstream:
  base_url: "https://api.example.com"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "logging.color") {
		t.Fatalf("expected logging.color validation error, got: %v", err)
	}
}

func TestLoad_InvalidLoggingFileMaxSize(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := `auth:
  downstream_api_key: "fixed-key"
logging:
  output: "file"
  file:
    max_size_mb: -1
upstream:
  base_url: "https://api.example.com"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "logging.file.max_size_mb") {
		t.Fatalf("expected logging.file.max_size_mb validation error, got: %v", err)
	}
}
