package cli

import (
	"log/slog"
	"os"

	"codex-gateway/internal/config"
	"codex-gateway/internal/logging"
)

func newRootLogger(cfg config.LoggingConfig, workdir string) (*slog.Logger, error) {
	return logging.NewWithConfig(
		logging.Config{
			Level:  cfg.Level,
			Format: cfg.Format,
			Output: cfg.Output,
			Color:  cfg.Color,
			File: logging.FileConfig{
				Dir:        cfg.File.Dir,
				Name:       cfg.File.Name,
				MaxSizeMB:  cfg.File.MaxSizeMB,
				MaxBackups: cfg.File.MaxBackups,
				MaxAgeDays: cfg.File.MaxAgeDays,
				Compress:   cfg.File.Compress,
			},
		},
		logging.BuildOptions{
			Workdir: workdir,
			Stdout:  os.Stdout,
		},
	)
}
