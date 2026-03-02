package logging

import (
	"context"
	"testing"
)

func TestEnsureRequestID_PreservesIncomingValue(t *testing.T) {
	const in = "req-client-123"
	if got := EnsureRequestID(in); got != in {
		t.Fatalf("expected %q, got %q", in, got)
	}
}

func TestEnsureRequestID_GeneratesWhenMissing(t *testing.T) {
	got := EnsureRequestID("")
	if got == "" {
		t.Fatal("expected generated request id")
	}

	if got == "req-fallback" {
		t.Fatal("unexpected fallback request id")
	}
}

func TestWithRequestID_ContextRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-ctx-001")
	if got := RequestIDFromContext(ctx); got != "req-ctx-001" {
		t.Fatalf("expected request id in context, got %q", got)
	}
}
