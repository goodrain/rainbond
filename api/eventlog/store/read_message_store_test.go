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

func newReadMessageStoreForTest(fileStore FileStore) *readMessageStore {
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
	return messageStore
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

func TestReadEventBarrelEventStreamSnapshotAndLiveAreAtomic(t *testing.T) {
	history := &db.EventLogMessage{EventID: "event-1", Message: "same message"}
	live := &db.EventLogMessage{EventID: "event-1", Message: "same message"}
	fileStore := newBlockingReplayFileStore(history)
	barrel := newReadEventBarrelForTest(fileStore)

	type subscriptionResult struct {
		history []*db.EventLogMessage
		live    chan *db.EventLogMessage
	}
	subscription := make(chan subscriptionResult, 1)
	go func() {
		historyMessages, liveMessages := barrel.addEventStreamSubChan("sse-1")
		subscription <- subscriptionResult{history: historyMessages, live: liveMessages}
	}()
	<-fileStore.readStarted

	insertDone := make(chan struct{})
	go func() {
		barrel.insertMessage(live)
		close(insertDone)
	}()

	select {
	case <-fileStore.appendCalled:
		t.Fatal("live message was persisted before the SSE history snapshot completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(fileStore.releaseRead)
	result := <-subscription
	<-insertDone

	if len(result.history) != 1 || result.history[0].Message != "same message" {
		t.Fatalf("history = %#v, want one preserved message", result.history)
	}
	assertMessage(t, result.live, "same message")
	select {
	case message := <-result.live:
		t.Fatalf("received duplicate message across history/live boundary: %#v", message)
	case <-time.After(50 * time.Millisecond):
	}

	barrel.delSubChan("sse-1")
	if _, ok := <-result.live; ok {
		t.Fatal("SSE live channel remains open after release")
	}
}

func TestReadEventBarrelLegacySubscriptionStillReplaysThroughChannel(t *testing.T) {
	history := &db.EventLogMessage{EventID: "event-1", Message: "legacy history"}
	fileStore := newBlockingReplayFileStore(history)
	barrel := newReadEventBarrelForTest(fileStore)

	subscription := make(chan chan *db.EventLogMessage, 1)
	go func() {
		subscription <- barrel.addSubChan("websocket-1")
	}()
	<-fileStore.readStarted
	close(fileStore.releaseRead)
	legacyMessages := <-subscription

	assertMessage(t, legacyMessages, "legacy history")
	barrel.delSubChan("websocket-1")
}

func TestStoreManagerKeepsEventStreamAndLegacySubscriptionsSeparate(t *testing.T) {
	history := &db.EventLogMessage{EventID: "event-1", Message: "history"}
	fileStore := newBlockingReplayFileStore(history)
	close(fileStore.releaseRead)
	messageStore := newReadMessageStoreForTest(fileStore)
	manager := &storeManager{readMessageStore: messageStore}

	streamHistory, streamLive := manager.EventStreamMessageChan("event-1", "sse-1")
	if len(streamHistory) != 1 || streamHistory[0].Message != "history" {
		t.Fatalf("SSE history = %#v, want one history message", streamHistory)
	}
	select {
	case message := <-streamLive:
		t.Fatalf("SSE live channel unexpectedly replayed history: %#v", message)
	default:
	}
	manager.ReleaseEventStreamMessageChan("event-1", "sse-1")
	if _, ok := <-streamLive; ok {
		t.Fatal("SSE live channel remains open after release")
	}

	legacy := manager.WebSocketMessageChan("event", "event-1", "websocket-1")
	assertMessage(t, legacy, "history")
	manager.RealseWebSocketMessageChan("event", "event-1", "websocket-1")
	if _, ok := <-legacy; ok {
		t.Fatal("legacy WebSocket channel remains open after release")
	}
}

func TestReadMessageStoreEventStreamSubscriptionIsSafeFromGC(t *testing.T) {
	fileStore := newBlockingReplayFileStore()
	messageStore := newReadMessageStoreForTest(fileStore)

	type subscriptionResult struct {
		history []*db.EventLogMessage
		live    chan *db.EventLogMessage
	}
	subscription := make(chan subscriptionResult, 1)
	go func() {
		history, live := messageStore.EventStreamMessageChan("event-1", "sse-1")
		subscription <- subscriptionResult{history: history, live: live}
	}()
	<-fileStore.readStarted

	messageStore.lock.Lock()
	barrel := messageStore.barrels["event-1"]
	activeUsers := barrel.activeUsers
	messageStore.lock.Unlock()
	if activeUsers != 1 {
		t.Fatalf("active users during replay = %d, want 1 so GC cannot recycle the barrel", activeUsers)
	}

	close(fileStore.releaseRead)
	result := <-subscription
	messageStore.lock.Lock()
	activeUsers = barrel.activeUsers
	currentBarrel := messageStore.barrels["event-1"]
	messageStore.lock.Unlock()
	if activeUsers != 0 {
		t.Fatalf("active users after subscription = %d, want 0", activeUsers)
	}
	if currentBarrel != barrel {
		t.Fatal("event barrel was replaced while the SSE subscription was being registered")
	}
	barrel.subLock.Lock()
	_, registered := barrel.subSocketChan["sse-1"]
	barrel.subLock.Unlock()
	if !registered {
		t.Fatal("SSE subscriber was not registered before the barrel became eligible for GC checks")
	}

	messageStore.RealseSubChan("event-1", "sse-1")
	if _, ok := <-result.live; ok {
		t.Fatal("SSE live channel remains open after release")
	}
}

func TestReadMessageStoreReplayDoesNotBlockOtherEvents(t *testing.T) {
	fileStore := newBlockingReplayFileStore()
	messageStore := newReadMessageStoreForTest(fileStore)

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
