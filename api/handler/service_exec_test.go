// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package handler

import (
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	k8sexec "k8s.io/client-go/util/exec"
)

func TestClampExecTimeout(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		want    time.Duration
	}{
		{name: "zero falls back to default", seconds: 0, want: PodExecDefaultTimeoutSeconds * time.Second},
		{name: "negative falls back to default", seconds: -5, want: PodExecDefaultTimeoutSeconds * time.Second},
		{name: "within range kept", seconds: 45, want: 45 * time.Second},
		{name: "over max clamped", seconds: 9999, want: PodExecMaxTimeoutSeconds * time.Second},
		{name: "exactly max kept", seconds: PodExecMaxTimeoutSeconds, want: PodExecMaxTimeoutSeconds * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampExecTimeout(tt.seconds); got != tt.want {
				t.Fatalf("clampExecTimeout(%d) = %v, want %v", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestCapBufferUnderLimit(t *testing.T) {
	// Arrange
	cb := newCapBuffer(16)

	// Act
	n, err := cb.Write([]byte("hello"))

	// Assert
	if err != nil {
		t.Fatalf("write err: %v", err)
	}
	if n != 5 {
		t.Fatalf("n = %d, want 5", n)
	}
	if cb.truncated {
		t.Fatalf("truncated = true, want false")
	}
	if cb.String() != "hello" {
		t.Fatalf("String() = %q, want hello", cb.String())
	}
}

func TestCapBufferExactlyAtLimit(t *testing.T) {
	cb := newCapBuffer(5)
	cb.Write([]byte("hello"))
	if cb.truncated {
		t.Fatalf("truncated = true at exact limit, want false")
	}
	if cb.String() != "hello" {
		t.Fatalf("String() = %q, want hello", cb.String())
	}
}

func TestCapBufferTruncatesSingleWrite(t *testing.T) {
	cb := newCapBuffer(4)
	n, _ := cb.Write([]byte("abcdefgh"))
	if n != 8 {
		t.Fatalf("reported written = %d, want 8 (full input length)", n)
	}
	if !cb.truncated {
		t.Fatalf("truncated = false, want true")
	}
	if cb.String() != "abcd" {
		t.Fatalf("String() = %q, want abcd", cb.String())
	}
}

func TestCapBufferTruncatesAcrossWrites(t *testing.T) {
	cb := newCapBuffer(5)
	cb.Write([]byte("abc"))
	cb.Write([]byte("def")) // only "de" fits, rest dropped
	if !cb.truncated {
		t.Fatalf("truncated = false, want true")
	}
	if cb.String() != "abcde" {
		t.Fatalf("String() = %q, want abcde", cb.String())
	}
	// A further write once full should keep truncated true and not grow.
	cb.Write([]byte("xyz"))
	if cb.String() != "abcde" {
		t.Fatalf("String() grew past limit: %q", cb.String())
	}
}

func TestCapBufferRespectsOneMiBConstant(t *testing.T) {
	cb := newCapBuffer(PodExecMaxOutputBytes)
	big := strings.Repeat("x", PodExecMaxOutputBytes+100)
	cb.Write([]byte(big))
	if !cb.truncated {
		t.Fatalf("truncated = false for oversize write, want true")
	}
	if len(cb.String()) != PodExecMaxOutputBytes {
		t.Fatalf("buffered len = %d, want %d", len(cb.String()), PodExecMaxOutputBytes)
	}
}

// TestCodeExitErrorExtraction verifies the exit-code extraction contract used
// by ExecCommand: a CodeExitError surfaces its numeric exit status via
// errors.As, and a generic error does not.
func TestCodeExitErrorExtraction(t *testing.T) {
	t.Run("CodeExitError yields exit status", func(t *testing.T) {
		var wrapped error = errors.Wrap(k8sexec.CodeExitError{Err: errors.New("command failed"), Code: 137}, "stream")
		var codeErr k8sexec.CodeExitError
		if !errors.As(wrapped, &codeErr) {
			t.Fatalf("errors.As did not match CodeExitError")
		}
		if codeErr.ExitStatus() != 137 {
			t.Fatalf("ExitStatus() = %d, want 137", codeErr.ExitStatus())
		}
	})
	t.Run("generic error does not match", func(t *testing.T) {
		var codeErr k8sexec.CodeExitError
		if errors.As(errors.New("network blip"), &codeErr) {
			t.Fatalf("errors.As matched a non-CodeExitError")
		}
	})
}

func TestErrContainerNotRunningIsDistinguishable(t *testing.T) {
	wrapped := errors.Wrapf(ErrContainerNotRunning, "container %s", "main")
	if !errors.Is(wrapped, ErrContainerNotRunning) {
		t.Fatalf("errors.Is failed to identify ErrContainerNotRunning")
	}
}
