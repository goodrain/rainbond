package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/barnettZQG/gotty/server"
	"github.com/barnettZQG/gotty/webtty"
	"github.com/gorilla/websocket"
)

const testTerminalWindow = 64 * 1024

type terminalFlowTestSlave struct {
	data      []byte
	offset    int
	closed    chan struct{}
	closeOnce sync.Once
	input     chan []byte
	readDone  chan struct{}
}

func (s *terminalFlowTestSlave) Read(p []byte) (int, error) {
	if s.offset < len(s.data) {
		n := copy(p, s.data[s.offset:])
		s.offset += n
		return n, nil
	}
	defer close(s.readDone)
	<-s.closed
	return 0, io.EOF
}

func (s *terminalFlowTestSlave) Write(p []byte) (int, error) {
	s.input <- append([]byte(nil), p...)
	return len(p), nil
}

func (s *terminalFlowTestSlave) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}

func (s *terminalFlowTestSlave) WindowTitleVariables() map[string]interface{} { return nil }
func (s *terminalFlowTestSlave) ResizeTerminal(columns, rows int) error       { return nil }

type terminalFlowTestSession struct {
	conn                *websocket.Conn
	frames              chan []byte
	errors              chan error
	done                chan struct{}
	readDone            chan struct{}
	cancel              context.CancelFunc
	slave               *terminalFlowTestSlave
	preferencesReceived bool
}

func startTerminalFlowTestSession(t *testing.T, enabled bool, data []byte) *terminalFlowTestSession {
	t.Helper()
	slave := &terminalFlowTestSlave{
		data: data, closed: make(chan struct{}), input: make(chan []byte, 4), readDone: make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &terminalFlowTestSession{
		frames: make(chan []byte, len(data)/1024+32), errors: make(chan error, 1), done: make(chan struct{}),
		readDone: make(chan struct{}), cancel: cancel, slave: slave,
	}
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(s.done)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		defer slave.Close()
		defer cancel()
		master, controlledSlave, options := configureTerminalFlowControl(ctx, &WsWrapper{conn}, slave, enabled)
		options = append(options, webtty.WithPermitWrite())
		tty, err := webtty.New(master, controlledSlave, options...)
		if err == nil {
			err = tty.Run(ctx)
		}
		s.errors <- err
	}))
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		cancel()
		server.Close()
		t.Fatal(err)
	}
	s.conn = conn
	go func() {
		defer close(s.readDone)
		for {
			_, frame, err := conn.ReadMessage()
			if err != nil {
				return
			}
			s.frames <- frame
		}
	}()
	t.Cleanup(func() {
		cancel()
		conn.Close()
		select {
		case <-s.done:
		case <-time.After(3 * time.Second):
			t.Error("terminal server did not exit after cancellation")
		}
		server.Close()
		select {
		case <-s.readDone:
		case <-time.After(3 * time.Second):
			t.Error("test websocket reader did not exit")
		}
	})
	return s
}

func (s *terminalFlowTestSession) next(t *testing.T) []byte {
	t.Helper()
	select {
	case frame := <-s.frames:
		return frame
	case <-time.After(3 * time.Second):
		t.Fatal("terminal frame timed out")
		return nil
	}
}

func (s *terminalFlowTestSession) send(t *testing.T, data string) {
	t.Helper()
	if err := s.conn.WriteMessage(websocket.TextMessage, []byte(data)); err != nil {
		t.Fatal(err)
	}
}

func (s *terminalFlowTestSession) output(t *testing.T, size int, expectPreferences bool) []byte {
	t.Helper()
	var data []byte
	for len(data) < size {
		frame := s.next(t)
		if len(frame) == 0 {
			t.Fatal("unexpected empty terminal frame")
		}
		switch frame[0] {
		case webtty.Output:
			decoded, err := base64.StdEncoding.DecodeString(string(frame[1:]))
			if err != nil {
				t.Fatal(err)
			}
			data = append(data, decoded...)
		case webtty.SetPreferences:
			if !expectPreferences {
				t.Fatal("legacy client received flow-control preferences")
			}
			var prefs map[string]interface{}
			if err := json.Unmarshal(frame[1:], &prefs); err != nil {
				t.Fatal(err)
			}
			if prefs["flowControl"] != true || prefs["flowControlWindow"] != float64(testTerminalWindow) {
				t.Fatalf("unexpected flow-control preferences: %s", frame)
			}
			s.preferencesReceived = true
		case webtty.SetWindowTitle, webtty.SetReconnect:
		default:
			t.Fatalf("unexpected frame: %q", frame)
		}
	}
	if len(data) != size {
		t.Fatalf("output exceeded window: got %d, want %d", len(data), size)
	}
	if expectPreferences && !s.preferencesReceived {
		t.Fatal("flow-controlled output arrived before its capability advertisement")
	}
	return data
}

// capability_id: rainbond.webcli.output-flow-control
func TestTerminalFlowControlBoundsOutputAndKeepsInputAlive(t *testing.T) {
	data := bytes.Repeat([]byte("终端🙂\n"), 20000)
	s := startTerminalFlowTestSession(t, true, data)
	got := s.output(t, testTerminalWindow, true)
	s.send(t, "2")
	if frame := s.next(t); string(frame) != "2" {
		t.Fatalf("expected Pong while output is full, got %q", frame)
	}
	s.send(t, "1hello")
	select {
	case input := <-s.slave.input:
		if string(input) != "hello" {
			t.Fatalf("unexpected terminal input: %q", input)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("input stalled behind a full output window")
	}
	for len(got) < len(data) {
		s.send(t, "4"+strconv.Itoa(testTerminalWindow))
		size := len(data) - len(got)
		if size > testTerminalWindow {
			size = testTerminalWindow
		}
		got = append(got, s.output(t, size, true)...)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("raw UTF-8 output changed across acknowledgement windows")
	}
}

// capability_id: rainbond.webcli.output-flow-control
func TestTerminalFlowControlLegacyClientDoesNotWaitForAcknowledgements(t *testing.T) {
	data := bytes.Repeat([]byte("legacy\n"), 20000)
	s := startTerminalFlowTestSession(t, false, data)
	if got := s.output(t, len(data), false); !bytes.Equal(got, data) {
		t.Fatal("legacy terminal output changed")
	}
}

// capability_id: rainbond.webcli.output-flow-control
func TestTerminalFlowControlPartialAcknowledgementOnlyReleasesConsumedBytes(t *testing.T) {
	s := startTerminalFlowTestSession(t, true, bytes.Repeat([]byte("a"), testTerminalWindow*2))
	s.output(t, testTerminalWindow, true)
	s.send(t, "4101")
	s.output(t, 101, true)
	s.send(t, "2")
	if frame := s.next(t); string(frame) != "2" {
		t.Fatalf("more than 101 bytes resumed without acknowledgement: %q", frame)
	}
}

// capability_id: rainbond.webcli.output-flow-control
func TestTerminalFlowControlCancellationWhileWindowFull(t *testing.T) {
	s := startTerminalFlowTestSession(t, true, bytes.Repeat([]byte("a"), testTerminalWindow*2))
	s.output(t, testTerminalWindow, true)
	s.cancel()
	select {
	case <-s.done:
	case <-time.After(3 * time.Second):
		t.Fatal("full output window prevented cancellation")
	}
}

// capability_id: rainbond.webcli.output-flow-control
func TestTerminalFlowControlDisconnectWhileWindowFull(t *testing.T) {
	s := startTerminalFlowTestSession(t, true, bytes.Repeat([]byte("a"), testTerminalWindow*2))
	s.output(t, testTerminalWindow, true)
	s.conn.Close()
	select {
	case <-s.done:
	case <-time.After(3 * time.Second):
		t.Fatal("client disconnect did not stop the terminal session")
	}
	select {
	case <-s.slave.closed:
	default:
		t.Fatal("PTY slave was not closed after client disconnect")
	}
}

// capability_id: rainbond.webcli.output-flow-control
func TestTerminalFlowControlRejectsInvalidAcknowledgements(t *testing.T) {
	for _, ack := range []string{"4", "40", "4-1", "4+1", "41.0", "4abc", "465537", "410000000000000000000000000000"} {
		t.Run(ack, func(t *testing.T) {
			s := startTerminalFlowTestSession(t, true, bytes.Repeat([]byte("a"), testTerminalWindow*2))
			s.output(t, testTerminalWindow, true)
			s.send(t, ack)
			select {
			case <-s.done:
			case <-time.After(3 * time.Second):
				t.Fatal("invalid acknowledgement did not close the terminal")
			}
		})
	}
}

// capability_id: rainbond.webcli.output-flow-control
func TestTerminalFlowControlRejectsAcknowledgementWithoutOutstandingOutput(t *testing.T) {
	s := startTerminalFlowTestSession(t, true, nil)
	for {
		frame := s.next(t)
		if frame[0] == webtty.SetPreferences {
			break
		}
	}
	s.send(t, "41")
	select {
	case <-s.done:
	case <-time.After(3 * time.Second):
		t.Fatal("acknowledgement created unearned output credit")
	}
}

// capability_id: rainbond.webcli.output-flow-control
func TestTerminalFlowControlCloseUnblocksWaitingSlaveRead(t *testing.T) {
	slave := &terminalFlowTestSlave{closed: make(chan struct{}), input: make(chan []byte, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, controlled, _ := configureTerminalFlowControl(ctx, &bytes.Buffer{}, slave, true)
	flowSlave := controlled.(*flowControlledTerminalSlave)
	flowSlave.flow.record(testTerminalWindow)
	done := make(chan error, 1)
	go func() {
		_, err := controlled.Read(make([]byte, 1024))
		done <- err
	}()
	controlled.Close()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("unexpected read error after close: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("closing the slave did not release the output-window waiter")
	}
}

// capability_id: rainbond.webcli.output-flow-control
func TestTerminalFlowControlInitFlagIsOptIn(t *testing.T) {
	for _, tc := range []struct {
		json string
		want bool
	}{
		{`{"C_id":"pod"}`, false},
		{`{"C_id":"pod","flow_control":false}`, false},
		{`{"C_id":"pod","flow_control":true}`, true},
	} {
		t.Run(tc.json, func(t *testing.T) {
			var init InitMessage
			if err := json.Unmarshal([]byte(tc.json), &init); err != nil {
				t.Fatal(err)
			}
			if init.FlowControl != tc.want {
				t.Fatalf("unexpected flow-control opt-in: %v", init.FlowControl)
			}
		})
	}
}

var _ server.Slave = (*terminalFlowTestSlave)(nil)
