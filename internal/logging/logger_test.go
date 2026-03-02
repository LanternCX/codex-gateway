package logging

import (
	"bytes"
	"context"
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

	if !strings.Contains(out, "level=INFO") {
		t.Fatalf("expected text level field, got: %s", out)
	}

	if !strings.Contains(out, "msg=\"request completed\"") {
		t.Fatalf("expected text message field, got: %s", out)
	}
}

func TestNew_InvalidLevel(t *testing.T) {
	if _, err := New("verbose", "text", nil); err == nil {
		t.Fatal("expected invalid level error")
	}
}
