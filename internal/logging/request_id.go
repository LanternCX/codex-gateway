package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const RequestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	if strings.TrimSpace(requestID) == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return strings.TrimSpace(value)
}

func EnsureRequestID(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed != "" {
		return trimmed
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "req-fallback"
	}

	return hex.EncodeToString(b)
}
