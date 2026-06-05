package handler

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// capability_id: rainbond.service.file-manage-command-safety
func TestBuildFileManageListCommand(t *testing.T) {
	path := "-nas/book/45256 supplement"

	got := buildFileManageListCommand(path)
	want := []string{"ls", "-1", "-p", "--", path}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected command: got %v want %v", got, want)
	}
}

// capability_id: rainbond.service.file-manage-exec-error-detail
func TestWrapFileManageExecErrorIncludesStderr(t *testing.T) {
	err := wrapFileManageExecError(
		"list directory",
		[]string{"ls", "-1", "-p", "--", "/nas/book/45256补充"},
		"ls: cannot access '/nas/book/45256补充': Permission denied",
		errors.New("command terminated with exit code 2"),
	)
	if err == nil {
		t.Fatal("expected wrapped error")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "Permission denied") {
		t.Fatalf("expected stderr in error, got %q", errMsg)
	}
	if !strings.Contains(errMsg, "ls -1 -p -- /nas/book/45256补充") {
		t.Fatalf("expected command in error, got %q", errMsg)
	}
}
