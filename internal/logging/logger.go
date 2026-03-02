package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

func New(level, format string, out io.Writer) (*slog.Logger, error) {
	if out == nil {
		out = os.Stdout
	}

	parsedLevel, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	options := &slog.HandlerOptions{Level: parsedLevel}
	handler, err := buildHandler(format, out, options)
	if err != nil {
		return nil, err
	}

	return slog.New(handler), nil
}

func parseLevel(in string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid logging level %q", in)
	}
}

func buildHandler(format string, out io.Writer, options *slog.HandlerOptions) (slog.Handler, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return slog.NewTextHandler(out, options), nil
	case "json":
		return slog.NewJSONHandler(out, options), nil
	default:
		return nil, fmt.Errorf("invalid logging format %q", format)
	}
}
