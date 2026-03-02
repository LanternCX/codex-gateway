package logging

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_JSONFormat(t *testing.T) {
	var buf bytes.Buffer

	logger, err := New("debug", "json", &buf)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	logger.InfoContext(context.Background(), "gateway started", "component", "cli")
	out := buf.String()

	if !strings.Contains(out, `"level":"INFO"`) {
		t.Fatalf("expected json level field, got: %s", out)
	}

	if !strings.Contains(out, `"msg":"gateway started"`) {
		t.Fatalf("expected json message field, got: %s", out)
	}
}

func TestNew_TextFormat(t *testing.T) {
	var buf bytes.Buffer

	logger, err := New("info", "text", &buf)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	logger.InfoContext(context.Background(), "request completed", "status", 200)
	out := buf.String()

	if !strings.Contains(out, " INFO ") {
		t.Fatalf("expected human-readable level token, got: %s", out)
	}

	if !strings.Contains(out, "request completed") {
		t.Fatalf("expected text message, got: %s", out)
	}

	if strings.Contains(out, "level=") || strings.Contains(out, "msg=") {
		t.Fatalf("expected human text format without key-value preamble, got: %s", out)
	}
}

func TestNew_InvalidLevel(t *testing.T) {
	if _, err := New("verbose", "text", nil); err == nil {
		t.Fatal("expected invalid level error")
	}
}

func TestNewWithConfig_FileOutputDefaultsToWorkdirLogs(t *testing.T) {
	workdir := t.TempDir()

	logger, err := NewWithConfig(Config{
		Level:  "info",
		Format: "text",
		Output: "file",
		File: FileConfig{
			Name:       "gateway.log",
			MaxSizeMB:  10,
			MaxBackups: 2,
			MaxAgeDays: 1,
			Compress:   false,
		},
	}, BuildOptions{Workdir: workdir})
	if err != nil {
		t.Fatalf("new logger with config: %v", err)
	}

	logger.InfoContext(context.Background(), "file sink ready")

	logPath := filepath.Join(workdir, "logs", "gateway.log")
	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	if !strings.Contains(string(b), "file sink ready") {
		t.Fatalf("expected log message in file, got: %s", string(b))
	}
}

func TestNewWithConfig_BothOutputWritesStdoutAndFile(t *testing.T) {
	workdir := t.TempDir()
	var stdout bytes.Buffer

	logger, err := NewWithConfig(Config{
		Level:  "info",
		Format: "json",
		Output: "both",
		File: FileConfig{
			Name:       "gateway.log",
			MaxSizeMB:  10,
			MaxBackups: 2,
			MaxAgeDays: 1,
			Compress:   false,
		},
	}, BuildOptions{Workdir: workdir, Stdout: &stdout})
	if err != nil {
		t.Fatalf("new logger with config: %v", err)
	}

	logger.InfoContext(context.Background(), "both sink ready")

	if !strings.Contains(stdout.String(), "both sink ready") {
		t.Fatalf("expected stdout output, got: %s", stdout.String())
	}

	b, err := os.ReadFile(filepath.Join(workdir, "logs", "gateway.log"))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	if !strings.Contains(string(b), "both sink ready") {
		t.Fatalf("expected file output, got: %s", string(b))
	}
}

func TestNewWithConfig_RedactsSensitiveValues(t *testing.T) {
	var stdout bytes.Buffer

	logger, err := NewWithConfig(Config{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}, BuildOptions{Stdout: &stdout})
	if err != nil {
		t.Fatalf("new logger with config: %v", err)
	}

	logger.InfoContext(context.Background(), "auth event", "authorization", "Bearer token-value", "access_token", "abc123")
	out := stdout.String()

	if strings.Contains(out, "token-value") || strings.Contains(out, "abc123") {
		t.Fatalf("expected sensitive values to be redacted, got: %s", out)
	}

	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected redacted marker, got: %s", out)
	}
}

func TestNewWithConfig_InvalidOutput(t *testing.T) {
	_, err := NewWithConfig(Config{
		Level:  "info",
		Format: "text",
		Output: "console",
	}, BuildOptions{})
	if err == nil {
		t.Fatal("expected invalid output error")
	}
}

func TestNewWithConfig_TextColorAlwaysAddsANSI(t *testing.T) {
	var stdout bytes.Buffer

	logger, err := NewWithConfig(Config{
		Level:  "info",
		Format: "text",
		Output: "stdout",
		Color:  "always",
	}, BuildOptions{Stdout: &stdout})
	if err != nil {
		t.Fatalf("new logger with color mode: %v", err)
	}

	logger.InfoContext(context.Background(), "colored log line")
	out := stdout.String()

	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected ANSI color code, got: %s", out)
	}
}

func TestNewWithConfig_TextColorNeverDisablesANSI(t *testing.T) {
	var stdout bytes.Buffer

	logger, err := NewWithConfig(Config{
		Level:  "info",
		Format: "text",
		Output: "stdout",
		Color:  "never",
	}, BuildOptions{Stdout: &stdout})
	if err != nil {
		t.Fatalf("new logger with color mode: %v", err)
	}

	logger.InfoContext(context.Background(), "plain log line")
	out := stdout.String()

	if strings.Contains(out, "\x1b[") {
		t.Fatalf("expected no ANSI color code, got: %s", out)
	}
}

func TestNewWithConfig_InvalidColorMode(t *testing.T) {
	_, err := NewWithConfig(Config{
		Level:  "info",
		Format: "text",
		Output: "stdout",
		Color:  "on",
	}, BuildOptions{})
	if err == nil {
		t.Fatal("expected invalid color mode error")
	}
}
