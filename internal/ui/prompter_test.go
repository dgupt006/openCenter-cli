// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ui

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestInteractivePrompter_Confirm_Yes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "lowercase y",
			input:    "y\n",
			expected: true,
		},
		{
			name:     "uppercase Y",
			input:    "Y\n",
			expected: true,
		},
		{
			name:     "lowercase yes",
			input:    "yes\n",
			expected: true,
		},
		{
			name:     "uppercase YES",
			input:    "YES\n",
			expected: true,
		},
		{
			name:     "mixed case Yes",
			input:    "Yes\n",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}
			prompter := NewInteractivePrompter(input, output)

			ctx := context.Background()
			result, err := prompter.Confirm(ctx, "Test message")

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
			if !strings.Contains(output.String(), "Test message") {
				t.Errorf("output should contain prompt message, got: %s", output.String())
			}
		})
	}
}

func TestInteractivePrompter_Confirm_No(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "lowercase n",
			input: "n\n",
		},
		{
			name:  "uppercase N",
			input: "N\n",
		},
		{
			name:  "lowercase no",
			input: "no\n",
		},
		{
			name:  "uppercase NO",
			input: "NO\n",
		},
		{
			name:  "random text",
			input: "maybe\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}
			prompter := NewInteractivePrompter(input, output)

			ctx := context.Background()
			result, err := prompter.Confirm(ctx, "Test message")

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != false {
				t.Errorf("expected false, got true")
			}
		})
	}
}

func TestInteractivePrompter_Confirm_ContextCancellation(t *testing.T) {
	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Create a prompter with a reader that blocks until the writer is closed
	reader, writer := io.Pipe()
	output := &bytes.Buffer{}
	prompter := NewInteractivePrompter(reader, output)

	// Cancel the context immediately while the open pipe keeps input blocked
	cancel()

	result, err := prompter.Confirm(ctx, "Test message")

	// Close both pipe ends deterministically after Confirm returns
	if closeErr := writer.Close(); closeErr != nil {
		t.Errorf("failed to close pipe writer: %v", closeErr)
	}
	if closeErr := reader.Close(); closeErr != nil {
		t.Errorf("failed to close pipe reader: %v", closeErr)
	}

	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}
	if result != false {
		t.Errorf("expected false result on cancellation, got true")
	}
}

func TestInteractivePrompter_Confirm_Timeout(t *testing.T) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Create a prompter with a reader that never provides input
	input := strings.NewReader("") // Empty reader will block
	output := &bytes.Buffer{}
	prompter := NewInteractivePrompter(input, output)

	result, err := prompter.Confirm(ctx, "Test message")

	// We expect either timeout or EOF error
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Accept either context.DeadlineExceeded or EOF as valid errors
	if err != context.DeadlineExceeded && !strings.Contains(err.Error(), "EOF") {
		t.Errorf("expected context.DeadlineExceeded or EOF error, got: %v", err)
	}
	if result != false {
		t.Errorf("expected false result on timeout, got true")
	}
}

func TestTestPrompter_Confirm_True(t *testing.T) {
	prompter := NewTestPrompter(true)
	ctx := context.Background()

	result, err := prompter.Confirm(ctx, "Test message")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != true {
		t.Errorf("expected true, got false")
	}
}

func TestTestPrompter_Confirm_False(t *testing.T) {
	prompter := NewTestPrompter(false)
	ctx := context.Background()

	result, err := prompter.Confirm(ctx, "Test message")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != false {
		t.Errorf("expected false, got true")
	}
}

func TestTestPrompter_Confirm_ContextCancellation(t *testing.T) {
	prompter := NewTestPrompter(true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := prompter.Confirm(ctx, "Test message")

	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}
	if result != false {
		t.Errorf("expected false result on cancellation, got true")
	}
}

func TestGetPrompter_TestMode(t *testing.T) {
	prompter := GetPrompter(nil, nil, true)

	// Should return a TestPrompter
	if _, ok := prompter.(*TestPrompter); !ok {
		t.Errorf("expected TestPrompter in test mode, got %T", prompter)
	}
}

func TestGetPrompter_InteractiveMode(t *testing.T) {
	input := strings.NewReader("y\n")
	output := &bytes.Buffer{}
	prompter := GetPrompter(input, output, false)

	// Should return an InteractivePrompter
	if _, ok := prompter.(*InteractivePrompter); !ok {
		t.Errorf("expected InteractivePrompter in interactive mode, got %T", prompter)
	}
}
