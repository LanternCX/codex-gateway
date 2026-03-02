package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
	OAuth    OAuthConfig    `yaml:"oauth"`
	Network  NetworkConfig  `yaml:"network"`
	Upstream UpstreamConfig `yaml:"upstream"`
}

type ServerConfig struct {
	Listen string `yaml:"listen"`
}

type AuthConfig struct {
	DownstreamAPIKey string `yaml:"downstream_api_key"`
}

type LoggingConfig struct {
	Level  string            `yaml:"level"`
	Format string            `yaml:"format"`
	Output string            `yaml:"output"`
	Color  string            `yaml:"color"`
	File   LoggingFileConfig `yaml:"file"`
}

type LoggingFileConfig struct {
	Dir        string `yaml:"dir"`
	Name       string `yaml:"name"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
	Compress   bool   `yaml:"compress"`
}

type OAuthConfig struct {
	ClientID                    string   `yaml:"client_id"`
	ClientSecret                string   `yaml:"client_secret"`
	AuthorizeEndpoint           string   `yaml:"authorize_endpoint"`
	DeviceAuthorizationEndpoint string   `yaml:"device_authorization_endpoint"`
	TokenEndpoint               string   `yaml:"token_endpoint"`
	RedirectHost                string   `yaml:"redirect_host"`
	RedirectPort                int      `yaml:"redirect_port"`
	RedirectPath                string   `yaml:"redirect_path"`
	Originator                  string   `yaml:"originator"`
	Scopes                      []string `yaml:"scopes"`
	Audience                    string   `yaml:"audience"`
}

type NetworkConfig struct {
	ProxyURL string `yaml:"proxy_url"`
}

type UpstreamConfig struct {
	BaseURL             string `yaml:"base_url"`
	ModelsPath          string `yaml:"models_path"`
	ChatCompletionsPath string `yaml:"chat_completions_path"`
	Mode                string `yaml:"mode"`
	CodexBaseURL        string `yaml:"codex_base_url"`
	CodexResponsesPath  string `yaml:"codex_responses_path"`
	TimeoutSeconds      int    `yaml:"timeout_seconds"`
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse yaml: %w", err)
	}

	applyDefaults(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Upstream.ModelsPath == "" {
		cfg.Upstream.ModelsPath = "/v1/models"
	}

	if cfg.Upstream.ChatCompletionsPath == "" {
		cfg.Upstream.ChatCompletionsPath = "/v1/chat/completions"
	}

	if cfg.Upstream.TimeoutSeconds == 0 {
		cfg.Upstream.TimeoutSeconds = 60
	}

	if cfg.Upstream.Mode == "" {
		cfg.Upstream.Mode = "codex_oauth"
	}

	if cfg.Upstream.CodexBaseURL == "" {
		cfg.Upstream.CodexBaseURL = "https://chatgpt.com"
	}

	if cfg.Upstream.CodexResponsesPath == "" {
		cfg.Upstream.CodexResponsesPath = "/backend-api/codex/responses"
	}

	if cfg.Server.Listen == "" {
		cfg.Server.Listen = ":8080"
	}

	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}

	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "text"
	}

	if cfg.Logging.Output == "" {
		cfg.Logging.Output = "stdout"
	}

	if cfg.Logging.Color == "" {
		cfg.Logging.Color = "auto"
	}

	if cfg.Logging.File.Name == "" {
		cfg.Logging.File.Name = "codex-gateway.log"
	}

	if cfg.Logging.File.MaxSizeMB == 0 {
		cfg.Logging.File.MaxSizeMB = 100
	}

	if cfg.Logging.File.MaxBackups == 0 {
		cfg.Logging.File.MaxBackups = 10
	}

	if cfg.Logging.File.MaxAgeDays == 0 {
		cfg.Logging.File.MaxAgeDays = 7
	}

	if cfg.OAuth.ClientID == "" {
		cfg.OAuth.ClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	}

	if cfg.OAuth.AuthorizeEndpoint == "" {
		cfg.OAuth.AuthorizeEndpoint = "https://auth.openai.com/oauth/authorize"
	}

	if cfg.OAuth.TokenEndpoint == "" {
		cfg.OAuth.TokenEndpoint = "https://auth.openai.com/oauth/token"
	}

	if cfg.OAuth.RedirectHost == "" {
		cfg.OAuth.RedirectHost = "localhost"
	}

	if cfg.OAuth.RedirectPort == 0 {
		cfg.OAuth.RedirectPort = 1455
	}

	if cfg.OAuth.RedirectPath == "" {
		cfg.OAuth.RedirectPath = "/auth/callback"
	}

	if cfg.OAuth.RedirectHost == "127.0.0.1" {
		cfg.OAuth.RedirectHost = "localhost"
	}

	if cfg.OAuth.Originator == "" {
		cfg.OAuth.Originator = "opencode"
	}

	if len(cfg.OAuth.Scopes) == 0 {
		cfg.OAuth.Scopes = []string{"openid", "profile", "email", "offline_access"}
	}
}

func (c Config) Validate() error {
	required := map[string]string{
		"auth.downstream_api_key": c.Auth.DownstreamAPIKey,
	}

	if c.Upstream.Mode == "openai_api" {
		required["upstream.base_url"] = c.Upstream.BaseURL
	}

	for key, val := range required {
		if strings.TrimSpace(val) == "" {
			return fmt.Errorf("missing required field: %s", key)
		}
	}

	level := strings.ToLower(strings.TrimSpace(c.Logging.Level))
	switch level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid logging.level %q (expected debug, info, warn, or error)", c.Logging.Level)
	}

	format := strings.ToLower(strings.TrimSpace(c.Logging.Format))
	switch format {
	case "text", "json":
	default:
		return fmt.Errorf("invalid logging.format %q (expected text or json)", c.Logging.Format)
	}

	output := strings.ToLower(strings.TrimSpace(c.Logging.Output))
	switch output {
	case "stdout", "file", "both":
	default:
		return fmt.Errorf("invalid logging.output %q (expected stdout, file, or both)", c.Logging.Output)
	}

	color := strings.ToLower(strings.TrimSpace(c.Logging.Color))
	switch color {
	case "auto", "always", "never":
	default:
		return fmt.Errorf("invalid logging.color %q (expected auto, always, or never)", c.Logging.Color)
	}

	if output != "stdout" {
		if strings.TrimSpace(c.Logging.File.Name) == "" {
			return fmt.Errorf("invalid logging.file.name %q (must not be empty)", c.Logging.File.Name)
		}

		if c.Logging.File.MaxSizeMB <= 0 {
			return fmt.Errorf("invalid logging.file.max_size_mb %d (must be > 0)", c.Logging.File.MaxSizeMB)
		}

		if c.Logging.File.MaxBackups <= 0 {
			return fmt.Errorf("invalid logging.file.max_backups %d (must be > 0)", c.Logging.File.MaxBackups)
		}

		if c.Logging.File.MaxAgeDays <= 0 {
			return fmt.Errorf("invalid logging.file.max_age_days %d (must be > 0)", c.Logging.File.MaxAgeDays)
		}
	}

	if proxyURL := strings.TrimSpace(c.Network.ProxyURL); proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil || !u.IsAbs() || strings.TrimSpace(u.Hostname()) == "" {
			return fmt.Errorf("invalid network.proxy_url %q (expected absolute URL with host)", c.Network.ProxyURL)
		}

		scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
		switch scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			return fmt.Errorf("invalid network.proxy_url %q (expected scheme http, https, socks5, or socks5h)", c.Network.ProxyURL)
		}
	}

	return nil
}
