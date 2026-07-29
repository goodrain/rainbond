package controller

import (
	"errors"
	"strings"
	"testing"
)

func TestVMOperationFailureMessageIncludesCause(t *testing.T) {
	msg := vmOperationFailureMessage("start", errors.New("virtualmachine instance already exists"))

	if !strings.Contains(msg, "start vm error") {
		t.Fatalf("expected VM action context in message, got %q", msg)
	}
	if !strings.Contains(msg, "virtualmachine instance already exists") {
		t.Fatalf("expected root cause in message, got %q", msg)
	}
}
