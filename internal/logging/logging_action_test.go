package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestActionLogWriter(t *testing.T) {
	ctx := context.Background()

	// When writer is not set, ActionLogWriter returns nil
	if writer := ActionLogWriter(ctx); writer != nil {
		t.Fatalf("expected nil writer, got: %v", writer)
	}

	// LogActionMessage with no writer does nothing and doesn't panic
	LogActionMessage(ctx, "test %s", "message")

	// Set writer
	var buf bytes.Buffer
	ctx = WithActionLogWriter(ctx, &buf)

	if writer := ActionLogWriter(ctx); writer == nil {
		t.Fatal("expected non-nil writer")
	}

	LogActionMessage(ctx, "hello %s", "world")

	logged := buf.String()
	if !strings.Contains(logged, "hello world") {
		t.Fatalf("expected log to contain 'hello world', got:\n%s", logged)
	}
}
