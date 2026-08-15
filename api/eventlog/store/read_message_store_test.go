// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

package store

import (
	"sync"
	"testing"
	"time"

	"github.com/goodrain/rainbond/api/eventlog/db"
)

type blockingReplayFileStore struct {
	lock sync.Mutex

	messages []*db.EventLogMessage

	readStarted  chan struct{}
	releaseRead  chan struct{}
	appendCalled chan struct{}
	readOnce     sync.Once
	appendOnce   sync.Once
}

func newBlockingReplayFileStore(messages ...*db.EventLogMessage) *blockingReplayFileStore {
	return &blockingReplayFileStore{
		messages:     append([]*db.EventLogMessage(nil), messages...),
		readStarted:  make(chan struct{}),
		releaseRead:  make(chan struct{}),
		appendCalled: make(chan struct{}),
	}
}

func (s *blockingReplayFileStore) Append(_ string, message *db.EventLogMessage) error {
	s.lock.Lock()
	s.messages = append(s.messages, message)
	s.lock.Unlock()
	s.appendOnce.Do(func() { close(s.appendCalled) })
	return nil
}

func (s *blockingReplayFileStore) ReadAll(string) ([]*db.EventLogMessage, error) {
	return s.readLast(0), nil
}

func (s *blockingReplayFileStore) ReadLast(_ string, n int) ([]*db.EventLogMessage, error) {
	s.readOnce.Do(func() { close(s.readStarted) })
	<-s.releaseRead
	return s.readLast(n), nil
}

func (s *blockingReplayFileStore) readLast(n int) []*db.EventLogMessage {
	s.lock.Lock()
	defer s.lock.Unlock()

	start := 0
	if n > 0 && len(s.messages) > n {
		start = len(s.messages) - n
	}
	return append([]*db.EventLogMessage(nil), s.messages[start:]...)
}

func (s *blockingReplayFileStore) Delete(string) error   { return nil }
func (s *blockingReplayFileStore) Clean(time.Time) error { return nil }
func (s *blockingReplayFileStore) Close() error          { return nil }

func newReadEventBarrelForTest(fileStore FileStore) *readEventBarrel {
	return &readEventBarrel{
		eventID:       "event-1",
		fileStore:     fileStore,
		subSocketChan: make(map[string]chan *db.EventLogMessage),
	}
}

func TestReadEventBarrelReplaysHistoryBeforeLiveWithoutDuplicates(t *testing.T) {
	history := &db.EventLogMessage{EventID: "event-1", Message: "history"}
	live := &db.EventLogMessage{EventID: "event-1", Message: "live"}
	fileStore := newBlockingReplayFileStore(history)
	barrel := newReadEventBarrelForTest(fileStore)

	subscription := make(chan chan *db.EventLogMessage, 1)
	go func() {
		subscription <- barrel.addSubChan("subscriber-1")
	}()
	<-fileStore.readStarted

	insertDone := make(chan struct{})
	go func() {
		barrel.insertMessage(live)
		close(insertDone)
	}()

	appendedBeforeReplayFinished := false
	select {
	case <-fileStore.appendCalled:
		appendedBeforeReplayFinished = true
	case <-time.After(100 * time.Millisecond):
	}

	close(fileStore.releaseRead)
	ch := <-subscription
	<-insertDone

	if appendedBeforeReplayFinished {
		t.Error("live message was persisted while history replay was still in progress")
	}

	assertMessage(t, ch, "history")
	assertMessage(t, ch, "live")
	select {
	case message := <-ch:
		t.Fatalf("received duplicate message after history replay: %#v", message)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestReadEventBarrelReleaseWaitsForHistoryReplay(t *testing.T) {
	fileStore := newBlockingReplayFileStore()
	barrel := newReadEventBarrelForTest(fileStore)

	subscription := make(chan chan *db.EventLogMessage, 1)
	go func() {
		subscription <- barrel.addSubChan("subscriber-1")
	}()
	<-fileStore.readStarted

	releaseDone := make(chan struct{})
	go func() {
		barrel.delSubChan("subscriber-1")
		close(releaseDone)
	}()

	releasedDuringReplay := false
	select {
	case <-releaseDone:
		releasedDuringReplay = true
	case <-time.After(100 * time.Millisecond):
	}

	close(fileStore.releaseRead)
	ch := <-subscription
	<-releaseDone

	if releasedDuringReplay {
		t.Error("subscriber channel was released while history replay was still in progress")
	}
	if _, ok := <-ch; ok {
		t.Error("subscriber channel remains open after release")
	}
}

func TestReadEventBarrelPreservesIdenticalLiveMessages(t *testing.T) {
	fileStore := newBlockingReplayFileStore()
	barrel := newReadEventBarrelForTest(fileStore)

	subscription := make(chan chan *db.EventLogMessage, 1)
	go func() {
		subscription <- barrel.addSubChan("subscriber-1")
	}()
	<-fileStore.readStarted
	close(fileStore.releaseRead)
	ch := <-subscription

	first := &db.EventLogMessage{EventID: "event-1", Message: "same message"}
	second := &db.EventLogMessage{EventID: "event-1", Message: "same message"}
	barrel.insertMessage(first)
	barrel.insertMessage(second)

	assertMessage(t, ch, "same message")
	assertMessage(t, ch, "same message")
}

func TestReadMessageStoreReplayDoesNotBlockOtherEvents(t *testing.T) {
	fileStore := newBlockingReplayFileStore()
	messageStore := &readMessageStore{
		barrels:   make(map[string]*readEventBarrel),
		fileStore: fileStore,
	}
	messageStore.pool = &sync.Pool{
		New: func() interface{} {
			return &readEventBarrel{
				subSocketChan: make(map[string]chan *db.EventLogMessage),
				fileStore:     fileStore,
			}
		},
	}

	subscription := make(chan chan *db.EventLogMessage, 1)
	go func() {
		subscription <- messageStore.SubChan("event-1", "subscriber-1")
	}()
	<-fileStore.readStarted

	insertDone := make(chan struct{})
	go func() {
		messageStore.InsertMessage(&db.EventLogMessage{EventID: "event-2", Message: "live"})
		close(insertDone)
	}()

	select {
	case <-insertDone:
	case <-time.After(time.Second):
		t.Error("history replay for one event blocked inserts for another event")
	}

	close(fileStore.releaseRead)
	ch := <-subscription
	messageStore.RealseSubChan("event-1", "subscriber-1")
	closed := make(chan struct{})
	go func() {
		for range ch {
		}
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Error("subscriber channel remains open after release")
	}
}

func assertMessage(t *testing.T, ch <-chan *db.EventLogMessage, want string) {
	t.Helper()
	select {
	case message, ok := <-ch:
		if !ok {
			t.Fatalf("subscriber channel closed before receiving %q", want)
		}
		if message == nil || message.Message != want {
			t.Fatalf("received message %#v, want %q", message, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for message %q", want)
	}
}
