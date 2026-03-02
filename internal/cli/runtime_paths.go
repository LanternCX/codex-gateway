package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type runtimePaths struct {
	Workdir    string
	ConfigPath string
	TokenPath  string
}

func resolveRuntimePaths(workdir, configFile string) (runtimePaths, error) {
	absWorkdir, err := filepath.Abs(strings.TrimSpace(workdir))
	if err != nil {
		return runtimePaths{}, fmt.Errorf("resolve workdir: %w", err)
	}

	info, err := os.Stat(absWorkdir)
	if err != nil {
		return runtimePaths{}, fmt.Errorf("stat workdir: %w", err)
	}
	if !info.IsDir() {
		return runtimePaths{}, fmt.Errorf("workdir is not a directory: %s", absWorkdir)
	}

	resolvedConfig := resolvePath(absWorkdir, configFile)
	absConfig, err := filepath.Abs(resolvedConfig)
	if err != nil {
		return runtimePaths{}, fmt.Errorf("resolve config path: %w", err)
	}

	if err := ensureWithinWorkdir(absWorkdir, absConfig); err != nil {
		return runtimePaths{}, err
	}

	return runtimePaths{
		Workdir:    absWorkdir,
		ConfigPath: absConfig,
		TokenPath:  filepath.Join(absWorkdir, "oauth-token.json"),
	}, nil
}

func ensureWithinWorkdir(workdir, target string) error {
	rel, err := filepath.Rel(workdir, target)
	if err != nil {
		return fmt.Errorf("check config path: %w", err)
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("config path is outside of workdir: %s", target)
	}

	return nil
}

func resolvePath(workdir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if workdir == "" {
		return path
	}
	return filepath.Join(workdir, path)
}
