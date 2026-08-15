package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	eventlogdb "github.com/goodrain/rainbond/api/eventlog/db"

	"golang.org/x/net/context"
)

type fakeEventMessageStore struct {
	messages chan *eventlogdb.EventLogMessage

	subscribeCalls int
	subscribeMode  string
	subscribeID    string
	subscriberID   string

	releaseCalls      int
	releaseMode       string
	releaseID         string
	releaseSubscriber string
}

func (f *fakeEventMessageStore) WebSocketMessageChan(mode, eventID, subID string) chan *eventlogdb.EventLogMessage {
	f.subscribeCalls++
	f.subscribeMode = mode
	f.subscribeID = eventID
	f.subscriberID = subID
	return f.messages
}

func (f *fakeEventMessageStore) RealseWebSocketMessageChan(mode, eventID, subID string) {
	f.releaseCalls++
	f.releaseMode = mode
	f.releaseID = eventID
	f.releaseSubscriber = subID
}

func (f *fakeEventMessageStore) assertReleasedOnce(t *testing.T, eventID string) {
	t.Helper()
	if f.subscribeCalls != 1 {
		t.Fatalf("subscribe calls = %d, want 1", f.subscribeCalls)
	}
	if f.subscribeMode != "event" || f.subscribeID != eventID || f.subscriberID == "" {
		t.Fatalf("unexpected subscription: mode=%q eventID=%q subscriberID=%q", f.subscribeMode, f.subscribeID, f.subscriberID)
	}
	if f.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", f.releaseCalls)
	}
	if f.releaseMode != f.subscribeMode || f.releaseID != f.subscribeID || f.releaseSubscriber != f.subscriberID {
		t.Fatalf("release does not match subscription: mode=%q eventID=%q subscriberID=%q", f.releaseMode, f.releaseID, f.releaseSubscriber)
	}
}

type responseWriterWithoutFlusher struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *responseWriterWithoutFlusher) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *responseWriterWithoutFlusher) WriteHeader(status int) {
	w.status = status
}

func (w *responseWriterWithoutFlusher) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

type failingStreamResponseWriter struct {
	header  http.Header
	status  int
	body    bytes.Buffer
	writes  int
	failAt  int
	flushes int
}

func (w *failingStreamResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingStreamResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *failingStreamResponseWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, errors.New("client disconnected")
	}
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func (w *failingStreamResponseWriter) Flush() {
	w.flushes++
}

func TestPushEventMessageSSERejectsEmptyEventID(t *testing.T) {
	server := &SocketServer{context: context.Background()}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v2/events//stream", nil)

	server.PushEventMessageSSE(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestStreamEventMessagesWritesHeadersAndCompleteJSONFrames(t *testing.T) {
	eventID := "event-1"
	message := &eventlogdb.EventLogMessage{
		EventID: eventID,
		Step:    "build",
		Status:  "running",
		Message: "building\nlayer 2",
		Level:   "debug",
		Time:    "2026-08-15T12:00:00+08:00",
		Content: []byte("legacy websocket payload"),
	}
	messages := make(chan *eventlogdb.EventLogMessage, 2)
	messages <- nil
	messages <- message
	close(messages)
	store := &fakeEventMessageStore{messages: messages}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	streamEventMessages(response, request, eventID, store, make(chan struct{}), time.Hour)

	wantJSON, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	wantBody := ": connected\n\ndata: " + string(wantJSON) + "\n\n"
	if response.Body.String() != wantBody {
		t.Fatalf("body = %q, want %q", response.Body.String(), wantBody)
	}
	wantHeaders := map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"Connection":        "keep-alive",
		"X-Accel-Buffering": "no",
	}
	for name, want := range wantHeaders {
		if got := response.Header().Get(name); got != want {
			t.Errorf("header %s = %q, want %q", name, got, want)
		}
	}
	if !response.Flushed {
		t.Fatal("response was not flushed")
	}
	store.assertReleasedOnce(t, eventID)
}

func TestStreamEventMessagesStopsAfterTerminalMessage(t *testing.T) {
	for _, terminalStep := range []string{"last", "callback"} {
		t.Run(terminalStep, func(t *testing.T) {
			eventID := "event-terminal-" + terminalStep
			terminal := &eventlogdb.EventLogMessage{EventID: eventID, Step: terminalStep, Status: "done", Message: "finished"}
			messages := make(chan *eventlogdb.EventLogMessage, 2)
			messages <- terminal
			messages <- &eventlogdb.EventLogMessage{EventID: eventID, Step: "after-terminal", Message: "must not be sent"}
			store := &fakeEventMessageStore{messages: messages}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/", nil)

			streamEventMessages(response, request, eventID, store, make(chan struct{}), time.Hour)

			terminalJSON, err := json.Marshal(terminal)
			if err != nil {
				t.Fatal(err)
			}
			wantBody := ": connected\n\ndata: " + string(terminalJSON) + "\n\n"
			if response.Body.String() != wantBody {
				t.Fatalf("body = %q, want %q", response.Body.String(), wantBody)
			}
			if len(messages) != 1 {
				t.Fatalf("messages left = %d, want 1", len(messages))
			}
			store.assertReleasedOnce(t, eventID)
		})
	}
}

func TestStreamEventMessagesSendsHeartbeat(t *testing.T) {
	eventID := "event-heartbeat"
	store := &fakeEventMessageStore{messages: make(chan *eventlogdb.EventLogMessage)}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	serverDone := make(chan struct{})
	time.AfterFunc(20*time.Millisecond, func() { close(serverDone) })

	streamEventMessages(response, request, eventID, store, serverDone, time.Millisecond)

	if !strings.Contains(response.Body.String(), ": ping\n\n") {
		t.Fatalf("body %q does not contain heartbeat", response.Body.String())
	}
	store.assertReleasedOnce(t, eventID)
}

func TestStreamEventMessagesExitsAndReleasesSubscription(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*http.Request, chan *eventlogdb.EventLogMessage, chan struct{}) *http.Request
	}{
		{
			name: "request canceled",
			configure: func(request *http.Request, _ chan *eventlogdb.EventLogMessage, _ chan struct{}) *http.Request {
				ctx, cancel := context.WithCancel(request.Context())
				cancel()
				return request.WithContext(ctx)
			},
		},
		{
			name: "server canceled",
			configure: func(request *http.Request, _ chan *eventlogdb.EventLogMessage, serverDone chan struct{}) *http.Request {
				close(serverDone)
				return request
			},
		},
		{
			name: "message channel closed",
			configure: func(request *http.Request, messages chan *eventlogdb.EventLogMessage, _ chan struct{}) *http.Request {
				close(messages)
				return request
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			eventID := "event-exit"
			messages := make(chan *eventlogdb.EventLogMessage)
			serverDone := make(chan struct{})
			request := test.configure(httptest.NewRequest(http.MethodGet, "/", nil), messages, serverDone)
			store := &fakeEventMessageStore{messages: messages}
			response := httptest.NewRecorder()

			streamEventMessages(response, request, eventID, store, serverDone, time.Hour)

			if response.Body.String() != ": connected\n\n" {
				t.Fatalf("body = %q, want connected frame only", response.Body.String())
			}
			store.assertReleasedOnce(t, eventID)
		})
	}
}

func TestStreamEventMessagesExitsOnWriteFailure(t *testing.T) {
	for _, test := range []struct {
		name   string
		failAt int
	}{
		{name: "initial frame", failAt: 1},
		{name: "data frame", failAt: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			eventID := "event-write-failure"
			messages := make(chan *eventlogdb.EventLogMessage, 1)
			messages <- &eventlogdb.EventLogMessage{EventID: eventID, Step: "build", Message: "building"}
			store := &fakeEventMessageStore{messages: messages}
			response := &failingStreamResponseWriter{failAt: test.failAt}
			request := httptest.NewRequest(http.MethodGet, "/", nil)

			streamEventMessages(response, request, eventID, store, make(chan struct{}), time.Hour)

			store.assertReleasedOnce(t, eventID)
		})
	}
}

func TestStreamEventMessagesRejectsInvalidStreamRequests(t *testing.T) {
	t.Run("empty event ID", func(t *testing.T) {
		store := &fakeEventMessageStore{messages: make(chan *eventlogdb.EventLogMessage)}
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/", nil)

		streamEventMessages(response, request, "", store, make(chan struct{}), time.Hour)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
		}
		if store.subscribeCalls != 0 || store.releaseCalls != 0 {
			t.Fatalf("invalid request subscribed %d times and released %d times", store.subscribeCalls, store.releaseCalls)
		}
	})

	t.Run("response writer without flusher", func(t *testing.T) {
		store := &fakeEventMessageStore{messages: make(chan *eventlogdb.EventLogMessage)}
		response := &responseWriterWithoutFlusher{}
		request := httptest.NewRequest(http.MethodGet, "/", nil)

		streamEventMessages(response, request, "event-no-flusher", store, make(chan struct{}), time.Hour)

		if response.status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", response.status, http.StatusInternalServerError)
		}
		if store.subscribeCalls != 0 || store.releaseCalls != 0 {
			t.Fatalf("invalid request subscribed %d times and released %d times", store.subscribeCalls, store.releaseCalls)
		}
	})
}
