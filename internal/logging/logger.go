package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	defaultOutput      = "stdout"
	defaultFormat      = "text"
	defaultColorMode   = "auto"
	defaultFileName    = "codex-gateway.log"
	defaultMaxSizeMB   = 100
	defaultMaxBackups  = 10
	defaultMaxAgeDays  = 7
	defaultLogsSubDir  = "logs"
	redactedValue      = "[REDACTED]"
	defaultWorkdirPath = "."

	ansiReset  = "\x1b[0m"
	ansiCyan   = "\x1b[36m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
)

var sensitiveKeys = map[string]struct{}{
	"authorization": {},
	"api_key":       {},
	"apikey":        {},
	"x_api_key":     {},
	"access_token":  {},
	"refresh_token": {},
	"client_secret": {},
	"password":      {},
	"secret":        {},
}

type Config struct {
	Level  string
	Format string
	Output string
	Color  string
	File   FileConfig
}

type FileConfig struct {
	Dir        string
	Name       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

type BuildOptions struct {
	Workdir string
	Stdout  io.Writer
}

func New(level, format string, out io.Writer) (*slog.Logger, error) {
	return NewWithConfig(
		Config{
			Level:  level,
			Format: format,
			Output: defaultOutput,
			Color:  defaultColorMode,
		},
		BuildOptions{Stdout: out},
	)
}

func NewWithConfig(cfg Config, opts BuildOptions) (*slog.Logger, error) {
	parsedLevel, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	output, err := parseOutput(cfg.Output)
	if err != nil {
		return nil, err
	}

	format, err := parseFormat(cfg.Format)
	if err != nil {
		return nil, err
	}

	colorMode, err := parseColorMode(cfg.Color)
	if err != nil {
		return nil, err
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	rewrite := composeAttrRewriters(redactSensitiveAttr)
	handlers := make([]slog.Handler, 0, 2)

	if output == "stdout" || output == "both" {
		handler, buildErr := newStdoutHandler(format, stdout, parsedLevel, rewrite, colorMode)
		if buildErr != nil {
			return nil, buildErr
		}
		handlers = append(handlers, handler)
	}

	if output == "file" || output == "both" {
		fileWriter, buildErr := newFileWriter(cfg.File, opts.Workdir)
		if buildErr != nil {
			return nil, buildErr
		}

		handler, buildErr := buildHandler(format, fileWriter, &slog.HandlerOptions{
			Level:       parsedLevel,
			ReplaceAttr: rewrite,
		})
		if buildErr != nil {
			return nil, buildErr
		}
		handlers = append(handlers, handler)
	}

	if len(handlers) == 0 {
		return nil, fmt.Errorf("no logging handlers configured")
	}

	return slog.New(newTeeHandler(handlers...)), nil
}

func newStdoutHandler(format string, out io.Writer, level slog.Level, rewrite func([]string, slog.Attr) slog.Attr, colorMode string) (slog.Handler, error) {
	if format == "text" {
		return newHumanTextHandler(out, level, rewrite, shouldColorizeLevel(colorMode, out)), nil
	}

	return buildHandler(format, out, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: rewrite,
	})
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

func parseOutput(in string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(in))
	if value == "" {
		value = defaultOutput
	}

	switch value {
	case "stdout", "file", "both":
		return value, nil
	default:
		return "", fmt.Errorf("invalid logging output %q", in)
	}
}

func parseFormat(in string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(in))
	if value == "" {
		value = defaultFormat
	}

	switch value {
	case "text", "json":
		return value, nil
	default:
		return "", fmt.Errorf("invalid logging format %q", in)
	}
}

func parseColorMode(in string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(in))
	if value == "" {
		value = defaultColorMode
	}

	switch value {
	case "auto", "always", "never":
		return value, nil
	default:
		return "", fmt.Errorf("invalid logging color mode %q", in)
	}
}

func newFileWriter(cfg FileConfig, workdir string) (io.Writer, error) {
	resolvedWorkdir := strings.TrimSpace(workdir)
	if resolvedWorkdir == "" {
		resolvedWorkdir = defaultWorkdirPath
	}

	resolvedDir := strings.TrimSpace(cfg.Dir)
	if resolvedDir == "" {
		resolvedDir = filepath.Join(resolvedWorkdir, defaultLogsSubDir)
	}

	if err := os.MkdirAll(resolvedDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = defaultFileName
	}

	maxSizeMB := cfg.MaxSizeMB
	if maxSizeMB <= 0 {
		maxSizeMB = defaultMaxSizeMB
	}

	maxBackups := cfg.MaxBackups
	if maxBackups <= 0 {
		maxBackups = defaultMaxBackups
	}

	maxAgeDays := cfg.MaxAgeDays
	if maxAgeDays <= 0 {
		maxAgeDays = defaultMaxAgeDays
	}

	return &lumberjack.Logger{
		Filename:   filepath.Join(resolvedDir, name),
		MaxSize:    maxSizeMB,
		MaxBackups: maxBackups,
		MaxAge:     maxAgeDays,
		Compress:   cfg.Compress,
	}, nil
}

type teeHandler struct {
	handlers []slog.Handler
}

func newTeeHandler(handlers ...slog.Handler) slog.Handler {
	if len(handlers) == 1 {
		return handlers[0]
	}
	return &teeHandler{handlers: handlers}
}

func (t *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range t.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (t *teeHandler) Handle(ctx context.Context, record slog.Record) error {
	var firstErr error
	for _, handler := range t.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}

		if err := handler.Handle(ctx, record.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (t *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, 0, len(t.handlers))
	for _, handler := range t.handlers {
		handlers = append(handlers, handler.WithAttrs(attrs))
	}
	return &teeHandler{handlers: handlers}
}

func (t *teeHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, 0, len(t.handlers))
	for _, handler := range t.handlers {
		handlers = append(handlers, handler.WithGroup(name))
	}
	return &teeHandler{handlers: handlers}
}

func redactSensitiveAttr(_ []string, attr slog.Attr) slog.Attr {
	if !isSensitiveKey(attr.Key) {
		return attr
	}

	return slog.String(attr.Key, redactedValue)
}

func composeAttrRewriters(rewriters ...func([]string, slog.Attr) slog.Attr) func([]string, slog.Attr) slog.Attr {
	if len(rewriters) == 0 {
		return nil
	}

	return func(groups []string, attr slog.Attr) slog.Attr {
		out := attr
		for _, rewriter := range rewriters {
			if rewriter == nil {
				continue
			}
			out = rewriter(groups, out)
		}
		return out
	}
}

func shouldColorizeLevel(colorMode string, out io.Writer) bool {
	switch colorMode {
	case "always":
		return true
	case "never":
		return false
	case "auto":
		return isTerminalWriter(out)
	default:
		return false
	}
}

func isTerminalWriter(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}

	return term.IsTerminal(int(file.Fd()))
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(key, "-", "_")))
	_, ok := sensitiveKeys[normalized]
	return ok
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

type humanTextHandler struct {
	out         io.Writer
	level       slog.Leveler
	replaceAttr func([]string, slog.Attr) slog.Attr
	attrs       []slog.Attr
	groups      []string
	colorize    bool
	mu          *sync.Mutex
}

type logField struct {
	key   string
	value string
}

func newHumanTextHandler(out io.Writer, level slog.Level, replaceAttr func([]string, slog.Attr) slog.Attr, colorize bool) slog.Handler {
	return &humanTextHandler{
		out:         out,
		level:       level,
		replaceAttr: replaceAttr,
		colorize:    colorize,
		mu:          &sync.Mutex{},
	}
}

func (h *humanTextHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *humanTextHandler) Handle(ctx context.Context, record slog.Record) error {
	if !h.Enabled(ctx, record.Level) {
		return nil
	}

	fields := make([]logField, 0, 8)
	appendField := func(attr slog.Attr, groups []string) {}
	appendField = func(attr slog.Attr, groups []string) {
		attr.Value = attr.Value.Resolve()
		if h.replaceAttr != nil {
			attr = h.replaceAttr(groups, attr)
		}

		if attr.Equal(slog.Attr{}) {
			return
		}

		if attr.Value.Kind() == slog.KindGroup {
			nextGroups := groups
			if attr.Key != "" {
				nextGroups = append(nextGroups, attr.Key)
			}
			for _, child := range attr.Value.Group() {
				appendField(child, nextGroups)
			}
			return
		}

		key := joinFieldKey(groups, attr.Key)
		if key == "" {
			return
		}

		fields = append(fields, logField{key: key, value: formatAttrValue(attr.Value)})
	}

	for _, attr := range h.attrs {
		appendField(attr, h.groups)
	}

	record.Attrs(func(attr slog.Attr) bool {
		appendField(attr, h.groups)
		return true
	})

	component := ""
	requestID := ""
	remaining := make([]logField, 0, len(fields))
	for _, field := range fields {
		switch field.key {
		case "component":
			component = field.value
		case "request_id":
			requestID = field.value
		default:
			remaining = append(remaining, field)
		}
	}

	sort.Slice(remaining, func(i, j int) bool {
		return remaining[i].key < remaining[j].key
	})

	timestamp := record.Time
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	level := strings.ToUpper(strings.TrimSpace(record.Level.String()))
	if h.colorize {
		level = formatLevelWithColor(level)
	}

	var builder strings.Builder
	builder.WriteString(timestamp.Format("2006-01-02 15:04:05.000"))
	builder.WriteByte(' ')
	builder.WriteString(level)
	if component != "" {
		builder.WriteString(" [")
		builder.WriteString(component)
		builder.WriteByte(']')
	}
	if requestID != "" {
		builder.WriteString(" [req=")
		builder.WriteString(requestID)
		builder.WriteByte(']')
	}
	if msg := strings.TrimSpace(record.Message); msg != "" {
		builder.WriteByte(' ')
		builder.WriteString(msg)
	}
	for _, field := range remaining {
		builder.WriteByte(' ')
		builder.WriteString(field.key)
		builder.WriteByte('=')
		builder.WriteString(field.value)
	}
	builder.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()

	_, err := io.WriteString(h.out, builder.String())
	return err
}

func (h *humanTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	combined = append(combined, h.attrs...)
	combined = append(combined, attrs...)

	return &humanTextHandler{
		out:         h.out,
		level:       h.level,
		replaceAttr: h.replaceAttr,
		attrs:       combined,
		groups:      append([]string{}, h.groups...),
		colorize:    h.colorize,
		mu:          h.mu,
	}
}

func (h *humanTextHandler) WithGroup(name string) slog.Handler {
	nextGroups := append([]string{}, h.groups...)
	if strings.TrimSpace(name) != "" {
		nextGroups = append(nextGroups, name)
	}

	return &humanTextHandler{
		out:         h.out,
		level:       h.level,
		replaceAttr: h.replaceAttr,
		attrs:       append([]slog.Attr{}, h.attrs...),
		groups:      nextGroups,
		colorize:    h.colorize,
		mu:          h.mu,
	}
}

func joinFieldKey(groups []string, key string) string {
	trimmed := strings.TrimSpace(key)
	if len(groups) == 0 {
		return trimmed
	}
	if trimmed == "" {
		return strings.Join(groups, ".")
	}
	return strings.Join(append(append([]string{}, groups...), trimmed), ".")
}

func formatAttrValue(value slog.Value) string {
	switch value.Kind() {
	case slog.KindString:
		return quoteIfNeeded(value.String())
	case slog.KindInt64:
		return strconv.FormatInt(value.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(value.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(value.Float64(), 'f', -1, 64)
	case slog.KindBool:
		return strconv.FormatBool(value.Bool())
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339Nano)
	case slog.KindAny:
		any := value.Any()
		switch v := any.(type) {
		case error:
			return quoteIfNeeded(v.Error())
		default:
			return quoteIfNeeded(fmt.Sprint(v))
		}
	default:
		return quoteIfNeeded(value.String())
	}
}

func quoteIfNeeded(value string) string {
	if value == "" {
		return `""`
	}

	if strings.ContainsAny(value, " \t\r\n\"=") {
		return strconv.Quote(value)
	}

	return value
}

func formatLevelWithColor(level string) string {
	switch {
	case strings.HasPrefix(level, "DEBUG"):
		return ansiCyan + level + ansiReset
	case strings.HasPrefix(level, "INFO"):
		return ansiGreen + level + ansiReset
	case strings.HasPrefix(level, "WARN"):
		return ansiYellow + level + ansiReset
	case strings.HasPrefix(level, "ERROR"):
		return ansiRed + level + ansiReset
	default:
		return level
	}
}
