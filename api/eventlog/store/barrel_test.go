package store

import (
	"sync"
	"testing"
	"time"

	"github.com/goodrain/rainbond/api/eventlog/db"
)

type delayedHistoryFileStore struct {
	mu          sync.Mutex
	readOnce    sync.Once
	appendOnce  sync.Once
	readStarted chan struct{}
	appendSeen  chan struct{}
	readDelay   time.Duration
	messages    []*db.EventLogMessage
}

func (s *delayedHistoryFileStore) Append(eventID string, message *db.EventLogMessage) error {
	s.mu.Lock()
	s.messages = append(s.messages, message)
	s.mu.Unlock()
	s.appendOnce.Do(func() {
		close(s.appendSeen)
	})
	return nil
}

func (s *delayedHistoryFileStore) ReadAll(eventID string) ([]*db.EventLogMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*db.EventLogMessage(nil), s.messages...), nil
}

func (s *delayedHistoryFileStore) ReadLast(eventID string, n int) ([]*db.EventLogMessage, error) {
	s.readOnce.Do(func() {
		close(s.readStarted)
	})
	select {
	case <-s.appendSeen:
	case <-time.After(s.readDelay):
	}
	return s.ReadAll(eventID)
}

func (s *delayedHistoryFileStore) Delete(eventID string) error {
	return nil
}

func (s *delayedHistoryFileStore) Clean(before time.Time) error {
	return nil
}

func (s *delayedHistoryFileStore) Close() error {
	return nil
}

func TestReadEventBarrelDoesNotReplayLiveMessageTwice(t *testing.T) {
	eventID := "build-event"
	store := &delayedHistoryFileStore{
		readStarted: make(chan struct{}),
		appendSeen:  make(chan struct{}),
		readDelay:   200 * time.Millisecond,
	}
	barrel := &readEventBarrel{
		eventID:       eventID,
		fileStore:     store,
		subSocketChan: make(map[string]chan *db.EventLogMessage),
	}

	ch := barrel.addSubChan("sub-1")
	select {
	case <-store.readStarted:
	case <-time.After(time.Second):
		t.Fatal("history replay did not start")
	}

	message := &db.EventLogMessage{
		EventID: eventID,
		Step:    "builder-exector",
		Message: "Build app version from source code start",
		Level:   "info",
	}
	inserted := make(chan struct{})
	go func() {
		barrel.insertMessage(message)
		close(inserted)
	}()

	select {
	case <-inserted:
	case <-time.After(time.Second):
		t.Fatal("live message insert blocked too long")
	}

	select {
	case got := <-ch:
		if got.Message != message.Message {
			t.Fatalf("expected %q, got %q", message.Message, got.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("expected live message")
	}

	select {
	case got := <-ch:
		t.Fatalf("message was delivered twice: %q", got.Message)
	case <-time.After(100 * time.Millisecond):
	}
}
