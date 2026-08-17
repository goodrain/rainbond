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
	eventlogstore "github.com/goodrain/rainbond/api/eventlog/store"

	"github.com/go-chi/chi"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
)

type legacyEventStoreManager struct {
	webSocketSubscribeCalls int
}

var _ eventlogstore.Manager = (*legacyEventStoreManager)(nil)

func (m *legacyEventStoreManager) ReceiveMessageChan() chan []byte    { return nil }
func (m *legacyEventStoreManager) SubMessageChan() chan [][]byte      { return nil }
func (m *legacyEventStoreManager) PubMessageChan() chan [][]byte      { return nil }
func (m *legacyEventStoreManager) DockerLogMessageChan() chan []byte  { return nil }
func (m *legacyEventStoreManager) GetDockerLogs(string, int) []string { return nil }
func (m *legacyEventStoreManager) MonitorMessageChan() chan [][]byte  { return nil }
func (m *legacyEventStoreManager) NewMonitorMessageChan() chan []byte { return nil }
func (m *legacyEventStoreManager) Run() error                         { return nil }
func (m *legacyEventStoreManager) Stop()                              {}
func (m *legacyEventStoreManager) Monitor() []eventlogdb.MonitorData  { return nil }
func (m *legacyEventStoreManager) Error() chan error                  { return nil }
func (m *legacyEventStoreManager) HealthCheck() map[string]string     { return nil }
func (m *legacyEventStoreManager) RealseWebSocketMessageChan(string, string, string) {
}
func (m *legacyEventStoreManager) WebSocketMessageChan(string, string, string) chan *eventlogdb.EventLogMessage {
	m.webSocketSubscribeCalls++
	return nil
}
func (m *legacyEventStoreManager) Scrape(chan<- prometheus.Metric, string, string, string) error {
	return nil
}

type fakeEventMessageStore struct {
	history  []*eventlogdb.EventLogMessage
	messages chan *eventlogdb.EventLogMessage

	subscribeCalls int
	subscribeMode  string
	subscribeID    string
	subscriberID   string

	releaseCalls      int
	releaseMode       string
	releaseID         string
	releaseSubscriber string

	streamSubscribeCalls int
	streamSubscribeID    string
	streamSubscriberID   string
	streamReleaseCalls   int
	streamReleaseID      string
	streamReleaseSubID   string
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

func (f *fakeEventMessageStore) EventStreamMessageChan(eventID, subID string) ([]*eventlogdb.EventLogMessage, chan *eventlogdb.EventLogMessage) {
	f.streamSubscribeCalls++
	f.streamSubscribeID = eventID
	f.streamSubscriberID = subID
	return f.history, f.messages
}

func (f *fakeEventMessageStore) ReleaseEventStreamMessageChan(eventID, subID string) {
	f.streamReleaseCalls++
	f.streamReleaseID = eventID
	f.streamReleaseSubID = subID
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

func (f *fakeEventMessageStore) assertEventStreamReleasedOnce(t *testing.T, eventID string) {
	t.Helper()
	if f.streamSubscribeCalls != 1 {
		t.Fatalf("stream subscribe calls = %d, want 1", f.streamSubscribeCalls)
	}
	if f.streamSubscribeID != eventID || f.streamSubscriberID == "" {
		t.Fatalf("unexpected stream subscription: eventID=%q subscriberID=%q", f.streamSubscribeID, f.streamSubscriberID)
	}
	if f.streamReleaseCalls != 1 {
		t.Fatalf("stream release calls = %d, want 1", f.streamReleaseCalls)
	}
	if f.streamReleaseID != f.streamSubscribeID || f.streamReleaseSubID != f.streamSubscriberID {
		t.Fatalf("stream release does not match subscription: eventID=%q subscriberID=%q", f.streamReleaseID, f.streamReleaseSubID)
	}
	if f.subscribeCalls != 0 || f.releaseCalls != 0 {
		t.Fatalf("SSE used legacy websocket subscription: subscribe=%d release=%d", f.subscribeCalls, f.releaseCalls)
	}
}

func eventDataFrame(t *testing.T, message *eventlogdb.EventLogMessage) string {
	t.Helper()
	payload, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	return "data: " + string(payload) + "\n\n"
}

const replayCompleteFrame = "event: replay-complete\ndata: {}\n\n"

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

type callbackStreamResponseWriter struct {
	header    http.Header
	body      bytes.Buffer
	writes    int
	callback  func()
	triggerAt int
}

func (w *callbackStreamResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *callbackStreamResponseWriter) WriteHeader(int) {}

func (w *callbackStreamResponseWriter) Write(data []byte) (int, error) {
	w.writes++
	written, err := w.body.Write(data)
	if w.writes == w.triggerAt {
		w.callback()
	}
	return written, err
}

func (w *callbackStreamResponseWriter) Flush() {}

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

func TestPushEventMessageSSERejectsManagerWithoutEventStreamCapability(t *testing.T) {
	manager := &legacyEventStoreManager{}
	server := &SocketServer{context: context.Background(), storemanager: manager}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v2/events/event-1/stream", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("eventID", "event-1")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))

	server.PushEventMessageSSE(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if response.Header().Get("Content-Type") == "text/event-stream" {
		t.Fatal("unsupported manager started an SSE response")
	}
	if manager.webSocketSubscribeCalls != 0 {
		t.Fatalf("legacy WebSocket subscription calls = %d, want 0", manager.webSocketSubscribeCalls)
	}
}

func TestStreamEventMessagesWritesHistoryBoundaryThenLive(t *testing.T) {
	eventID := "event-replay"
	history := []*eventlogdb.EventLogMessage{
		{EventID: eventID, Step: "build", Status: "running", Message: "same", Level: "debug", Time: "2026-08-16T10:00:00+08:00"},
		{EventID: eventID, Step: "build", Status: "running", Message: "same", Level: "debug", Time: "2026-08-16T10:00:00+08:00"},
	}
	live := []*eventlogdb.EventLogMessage{
		{EventID: eventID, Step: "build", Status: "running", Message: "same", Level: "debug", Time: "2026-08-16T10:00:01+08:00"},
		{EventID: eventID, Step: "build", Status: "running", Message: "same", Level: "debug", Time: "2026-08-16T10:00:01+08:00"},
	}
	messages := make(chan *eventlogdb.EventLogMessage, len(live))
	for _, message := range live {
		messages <- message
	}
	close(messages)
	store := &fakeEventMessageStore{history: history, messages: messages}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	streamEventMessages(response, request, eventID, store, make(chan struct{}), time.Hour)

	wantBody := ": connected\n\n"
	for _, message := range history {
		wantBody += eventDataFrame(t, message)
	}
	wantBody += replayCompleteFrame
	for _, message := range live {
		wantBody += eventDataFrame(t, message)
	}
	if response.Body.String() != wantBody {
		t.Fatalf("body = %q, want %q", response.Body.String(), wantBody)
	}
	if got := strings.Count(response.Body.String(), "event: replay-complete\n"); got != 1 {
		t.Fatalf("replay-complete frames = %d, want 1", got)
	}
	store.assertEventStreamReleasedOnce(t, eventID)
}

func TestStreamEventMessagesClosesAfterHistoricalTerminalAndBoundary(t *testing.T) {
	for _, terminalStep := range []string{"last", "callback"} {
		t.Run(terminalStep, func(t *testing.T) {
			eventID := "event-history-terminal-" + terminalStep
			terminal := &eventlogdb.EventLogMessage{EventID: eventID, Step: terminalStep, Status: "done", Message: "finished"}
			history := []*eventlogdb.EventLogMessage{
				terminal,
				{EventID: eventID, Step: "after-terminal", Message: "must not be sent"},
			}
			messages := make(chan *eventlogdb.EventLogMessage, 1)
			messages <- &eventlogdb.EventLogMessage{EventID: eventID, Step: "live", Message: "must not be sent"}
			store := &fakeEventMessageStore{history: history, messages: messages}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/", nil)

			streamEventMessages(response, request, eventID, store, make(chan struct{}), time.Hour)

			wantBody := ": connected\n\n" + eventDataFrame(t, terminal) + replayCompleteFrame
			if response.Body.String() != wantBody {
				t.Fatalf("body = %q, want %q", response.Body.String(), wantBody)
			}
			if len(messages) != 1 {
				t.Fatalf("live messages left = %d, want 1", len(messages))
			}
			store.assertEventStreamReleasedOnce(t, eventID)
		})
	}
}

func TestStreamEventMessagesStopsHistoryReplayImmediatelyWhenCanceled(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(*http.Request, chan struct{}) (*http.Request, func())
	}{
		{
			name: "request canceled",
			configure: func(request *http.Request, _ chan struct{}) (*http.Request, func()) {
				requestContext, cancel := context.WithCancel(request.Context())
				return request.WithContext(requestContext), cancel
			},
		},
		{
			name: "server canceled",
			configure: func(request *http.Request, serverDone chan struct{}) (*http.Request, func()) {
				return request, func() { close(serverDone) }
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			eventID := "event-history-cancel"
			history := []*eventlogdb.EventLogMessage{
				{EventID: eventID, Step: "build", Message: "first"},
				{EventID: eventID, Step: "build", Message: "must not be sent"},
			}
			store := &fakeEventMessageStore{history: history, messages: make(chan *eventlogdb.EventLogMessage)}
			serverDone := make(chan struct{})
			request, cancel := test.configure(httptest.NewRequest(http.MethodGet, "/", nil), serverDone)
			response := &callbackStreamResponseWriter{callback: cancel, triggerAt: 2}

			streamEventMessages(response, request, eventID, store, serverDone, time.Hour)

			wantBody := ": connected\n\n" + eventDataFrame(t, history[0])
			if response.body.String() != wantBody {
				t.Fatalf("body = %q, want %q", response.body.String(), wantBody)
			}
			if strings.Contains(response.body.String(), "replay-complete") {
				t.Fatal("replay boundary was written after cancellation")
			}
			store.assertEventStreamReleasedOnce(t, eventID)
		})
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
	wantBody := ": connected\n\n" + replayCompleteFrame + "data: " + string(wantJSON) + "\n\n"
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
	store.assertEventStreamReleasedOnce(t, eventID)
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
			wantBody := ": connected\n\n" + replayCompleteFrame + "data: " + string(terminalJSON) + "\n\n"
			if response.Body.String() != wantBody {
				t.Fatalf("body = %q, want %q", response.Body.String(), wantBody)
			}
			if len(messages) != 1 {
				t.Fatalf("messages left = %d, want 1", len(messages))
			}
			store.assertEventStreamReleasedOnce(t, eventID)
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
	store.assertEventStreamReleasedOnce(t, eventID)
}

func TestStreamEventMessagesExitsAndReleasesSubscription(t *testing.T) {
	tests := []struct {
		name               string
		configure          func(*http.Request, chan *eventlogdb.EventLogMessage, chan struct{}) *http.Request
		wantReplayBoundary bool
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
			name:               "message channel closed",
			wantReplayBoundary: true,
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

			wantBody := ": connected\n\n"
			if test.wantReplayBoundary {
				wantBody += replayCompleteFrame
			}
			if response.Body.String() != wantBody {
				t.Fatalf("body = %q, want %q", response.Body.String(), wantBody)
			}
			store.assertEventStreamReleasedOnce(t, eventID)
		})
	}
}

func TestStreamEventMessagesExitsOnWriteFailure(t *testing.T) {
	for _, test := range []struct {
		name     string
		failAt   int
		history  []*eventlogdb.EventLogMessage
		messages []*eventlogdb.EventLogMessage
	}{
		{name: "initial frame", failAt: 1},
		{
			name:    "history data frame",
			failAt:  2,
			history: []*eventlogdb.EventLogMessage{{EventID: "event-write-failure", Step: "build", Message: "history"}},
		},
		{name: "replay boundary", failAt: 2},
		{
			name:     "live data frame",
			failAt:   3,
			messages: []*eventlogdb.EventLogMessage{{EventID: "event-write-failure", Step: "build", Message: "live"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			eventID := "event-write-failure"
			messages := make(chan *eventlogdb.EventLogMessage, len(test.messages))
			for _, message := range test.messages {
				messages <- message
			}
			store := &fakeEventMessageStore{history: test.history, messages: messages}
			response := &failingStreamResponseWriter{failAt: test.failAt}
			request := httptest.NewRequest(http.MethodGet, "/", nil)

			streamEventMessages(response, request, eventID, store, make(chan struct{}), time.Hour)

			store.assertEventStreamReleasedOnce(t, eventID)
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
		if store.subscribeCalls != 0 || store.releaseCalls != 0 || store.streamSubscribeCalls != 0 || store.streamReleaseCalls != 0 {
			t.Fatalf("invalid request subscribed through legacy=%d stream=%d and released through legacy=%d stream=%d", store.subscribeCalls, store.streamSubscribeCalls, store.releaseCalls, store.streamReleaseCalls)
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
		if store.subscribeCalls != 0 || store.releaseCalls != 0 || store.streamSubscribeCalls != 0 || store.streamReleaseCalls != 0 {
			t.Fatalf("invalid request subscribed through legacy=%d stream=%d and released through legacy=%d stream=%d", store.subscribeCalls, store.streamSubscribeCalls, store.releaseCalls, store.streamReleaseCalls)
		}
	})
}
