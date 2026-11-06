package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
)

func TestWaitForGCExecutorRetriesOnlyStartupRace(t *testing.T) {
	calls := 0
	err := waitForGCExecutor(context.Background(), time.Millisecond, 20*time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return kubeidentity.ErrExecutorStarting
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatal("startup race was not retried", calls, err)
	}

	permanent := errors.New("changed executor")
	calls = 0
	err = waitForGCExecutor(context.Background(), time.Millisecond, 20*time.Millisecond, func() error {
		calls++
		return permanent
	})
	if !errors.Is(err, permanent) || calls != 1 {
		t.Fatal("permanent error was retried", calls, err)
	}
}
