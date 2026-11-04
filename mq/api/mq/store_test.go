package mq

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goodrain/rainbond/config/configs/rbdcomponent"
)

func newTestQueue() *etcdQueue {
	return &etcdQueue{
		client:   NewKeyValueStore(),
		mqConfig: &rbdcomponent.MQConfig{KeyPrefix: "/mq"},
	}
}

// capability_id: rainbond.mq.cancelable-dequeue
func TestDequeueCanceledRequestPreservesQueuedMessage(t *testing.T) {
	queue := newTestQueue()
	key := queue.queueKey("worker-health")
	queue.client.Put(key, "next health check")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	message, err := queue.Dequeue(ctx, "worker-health")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request consumed message %q: %v", message, err)
	}
	if queue.client.Size(key) != 1 {
		t.Fatal("canceled request removed a queued message")
	}
}

// capability_id: rainbond.mq.cancelable-dequeue
func TestDequeueHonorsCallerDeadline(t *testing.T) {
	queue := newTestQueue()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := queue.Dequeue(ctx, "worker-health")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected dequeue result: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("dequeue kept waiting after caller deadline: %v", elapsed)
	}
}

// capability_id: rainbond.mq.cancelable-dequeue
func TestCanceledDequeueDoesNotConsumeLaterMessage(t *testing.T) {
	queue := newTestQueue()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := queue.Dequeue(ctx, "worker-health")
		done <- err
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected canceled dequeue result: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("canceled dequeue is still waiting for later messages")
	}
	queue.client.Put(queue.queueKey("worker-health"), "next health check")
	message, err := queue.Dequeue(context.Background(), "worker-health")
	if err != nil || message != "next health check" {
		t.Fatalf("later health message was lost: message=%q error=%v", message, err)
	}
}

// capability_id: rainbond.mq.cancelable-dequeue
func TestDequeueWaitsForMessagesInFIFOOrder(t *testing.T) {
	queue := newTestQueue()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	type result struct {
		message string
		err     error
	}
	done := make(chan result, 1)
	go func() {
		message, err := queue.Dequeue(ctx, "worker")
		done <- result{message, err}
	}()
	time.Sleep(10 * time.Millisecond)
	queue.client.Put(queue.queueKey("worker"), "first task")
	queue.client.Put(queue.queueKey("worker"), "second task")
	first := <-done
	if first.err != nil || first.message != "first task" {
		t.Fatalf("unexpected first dequeue result: %+v", first)
	}
	second, err := queue.Dequeue(ctx, "worker")
	if err != nil || second != "second task" {
		t.Fatalf("FIFO order changed: message=%q error=%v", second, err)
	}
}
