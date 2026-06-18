package controller

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"testing"
	"unicode/utf8"
)

func TestParseFollowDefaultsToTrue(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/tenants/demo/services/svc/pods/pod-1/logs?lines=50", nil)

	follow, err := parseFollow(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !follow {
		t.Fatal("expected follow to default to true")
	}
}

func TestParseFollowParsesFalse(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/tenants/demo/services/svc/pods/pod-1/logs?lines=50&follow=false", nil)

	follow, err := parseFollow(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if follow {
		t.Fatal("expected follow to be false")
	}
}

func TestParseFollowRejectsInvalidValue(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/tenants/demo/services/svc/pods/pod-1/logs?follow=maybe", nil)

	if _, err := parseFollow(req); err == nil {
		t.Fatal("expected invalid follow value error")
	}
}

func TestParsePreviousDefaultsToFalse(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/tenants/demo/services/svc/pods/pod-1/logs?lines=50", nil)

	if parsePrevious(req) {
		t.Fatal("expected previous to default to false")
	}
}

func TestParsePreviousParsesTrue(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/tenants/demo/services/svc/pods/pod-1/logs?previous=true", nil)

	if !parsePrevious(req) {
		t.Fatal("expected previous to be true")
	}
}

func TestParsePreviousInvalidValueTreatedAsFalse(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/tenants/demo/services/svc/pods/pod-1/logs?previous=maybe", nil)

	if parsePrevious(req) {
		t.Fatal("expected invalid previous value to be treated as false")
	}
}

func TestSanitizeLogLineValidUTF8(t *testing.T) {
	input := "2024-01-01T00:00:00Z INFO Application started"
	result := sanitizeLogLine(input)
	if result != input {
		t.Errorf("expected %q, got %q", input, result)
	}
}

func TestSanitizeLogLineNonUTF8(t *testing.T) {
	// Invalid UTF-8 sequence: 0xFF 0xFE
	input := "log line with \xff\xfe invalid bytes"
	result := sanitizeLogLine(input)

	// The result should be valid UTF-8
	if !isValidUTF8(result) {
		t.Errorf("result is not valid UTF-8: %q", result)
	}

	// The result should contain the replacement character
	expectedContains := "�"
	if !containsString(result, expectedContains) {
		t.Errorf("expected result to contain replacement character, got %q", result)
	}
}

func TestSanitizeLogLineMixed(t *testing.T) {
	// Mix of valid and invalid UTF-8
	input := "valid \xff\xfe also valid"
	result := sanitizeLogLine(input)

	if !isValidUTF8(result) {
		t.Errorf("result is not valid UTF-8: %q", result)
	}

	// Should contain "valid " at the beginning and " also valid" at the end
	if !containsString(result, "valid ") || !containsString(result, " also valid") {
		t.Errorf("expected result to preserve valid parts, got %q", result)
	}
}

func TestSanitizeLogLineEmpty(t *testing.T) {
	result := sanitizeLogLine("")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestIsTransportErrorNil(t *testing.T) {
	if isTransportError(nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsTransportErrorNetError(t *testing.T) {
	err := &net.OpError{Op: "read", Err: errors.New("connection reset")}
	if !isTransportError(err) {
		t.Error("expected true for net.Error")
	}
}

func TestIsTransportErrorConnectionReset(t *testing.T) {
	err := errors.New("read tcp: connection reset by peer")
	if !isTransportError(err) {
		t.Error("expected true for connection reset error")
	}
}

func TestIsTransportErrorBrokenPipe(t *testing.T) {
	err := errors.New("write: broken pipe")
	if !isTransportError(err) {
		t.Error("expected true for broken pipe error")
	}
}

func TestIsTransportErrorClosedNetwork(t *testing.T) {
	err := errors.New("use of closed network connection")
	if !isTransportError(err) {
		t.Error("expected true for closed network connection error")
	}
}

func TestIsTransportErrorRegularError(t *testing.T) {
	err := errors.New("some other error")
	if isTransportError(err) {
		t.Error("expected false for regular error")
	}
}

func TestIsExpectedLogStreamCloseNil(t *testing.T) {
	if isExpectedLogStreamClose(context.Background(), nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsExpectedLogStreamCloseEOF(t *testing.T) {
	// Test actual io.EOF
	if !isExpectedLogStreamClose(context.Background(), io.EOF) {
		t.Error("expected true for io.EOF")
	}
}

func TestIsExpectedLogStreamCloseContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if !isExpectedLogStreamClose(ctx, errors.New("some error")) {
		t.Error("expected true when context is canceled")
	}
}

func TestIsExpectedLogStreamCloseNetClosed(t *testing.T) {
	// net.ErrClosed is the actual error we're checking for
	if !isExpectedLogStreamClose(context.Background(), net.ErrClosed) {
		t.Error("expected true for net.ErrClosed")
	}
}

// Helper functions for tests
func isValidUTF8(s string) bool {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			return false
		}
		i += size
	}
	return true
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
