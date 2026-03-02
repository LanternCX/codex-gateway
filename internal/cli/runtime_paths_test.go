package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRuntimePaths_ConfigInsideWorkdir(t *testing.T) {
	workdir := t.TempDir()
	configPath := filepath.Join(workdir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("auth:\n  downstream_api_key: test\nupstream:\n  base_url: https://api.example.com\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	paths, err := resolveRuntimePaths(workdir, "config.yaml")
	if err != nil {
		t.Fatalf("expected runtime paths, got error: %v", err)
	}

	if paths.Workdir != workdir {
		t.Fatalf("unexpected workdir: %q", paths.Workdir)
	}

	if paths.ConfigPath != configPath {
		t.Fatalf("unexpected config path: %q", paths.ConfigPath)
	}

	wantToken := filepath.Join(workdir, "oauth-token.json")
	if paths.TokenPath != wantToken {
		t.Fatalf("unexpected token path: got %q want %q", paths.TokenPath, wantToken)
	}
}

func TestResolveRuntimePaths_RejectsConfigOutsideWorkdir(t *testing.T) {
	root := t.TempDir()
	workdir := filepath.Join(root, "run")
	outside := filepath.Join(root, "outside")

	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}

	outsideConfig := filepath.Join(outside, "config.yaml")
	if err := os.WriteFile(outsideConfig, []byte("x: y\n"), 0o644); err != nil {
		t.Fatalf("write outside config: %v", err)
	}

	_, err := resolveRuntimePaths(workdir, outsideConfig)
	if err == nil {
		t.Fatal("expected error for config outside workdir")
	}

	if !strings.Contains(err.Error(), "outside of workdir") {
		t.Fatalf("expected outside workdir error, got: %v", err)
	}
}

func TestResolveRuntimePaths_RejectsTraversalOutsideWorkdir(t *testing.T) {
	root := t.TempDir()
	workdir := filepath.Join(root, "run")
	outside := filepath.Join(root, "outside")

	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}

	outsideConfig := filepath.Join(outside, "config.yaml")
	if err := os.WriteFile(outsideConfig, []byte("x: y\n"), 0o644); err != nil {
		t.Fatalf("write outside config: %v", err)
	}

	_, err := resolveRuntimePaths(workdir, "../outside/config.yaml")
	if err == nil {
		t.Fatal("expected traversal error")
	}

	if !strings.Contains(err.Error(), "outside of workdir") {
		t.Fatalf("expected outside workdir error, got: %v", err)
	}
}
