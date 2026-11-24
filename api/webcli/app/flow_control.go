package app

import (
	"context"
	"errors"
	"sync"

	"github.com/barnettZQG/gotty/server"
	"github.com/barnettZQG/gotty/webtty"
)

const terminalOutputWindow = 64 * 1024

// terminalOutputAck is negotiated through the initial request and preferences.
// It is consumed here because the legacy gotty protocol does not recognize it.
const terminalOutputAck = '4'

type terminalOutputFlow struct {
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	pending int
	changed chan struct{}
}

func (f *terminalOutputFlow) budget() (int, error) {
	for {
		select {
		case <-f.ctx.Done():
			return 0, f.ctx.Err()
		default:
		}
		f.mu.Lock()
		available := terminalOutputWindow - f.pending
		f.mu.Unlock()
		if available > 0 {
			return available, nil
		}
		select {
		case <-f.ctx.Done():
			return 0, f.ctx.Err()
		case <-f.changed:
		}
	}
}

func (f *terminalOutputFlow) record(n int) {
	f.mu.Lock()
	f.pending += n
	f.mu.Unlock()
}

func (f *terminalOutputFlow) acknowledge(payload []byte) error {
	if len(payload) == 0 || len(payload) > 5 {
		return errors.New("invalid terminal output acknowledgement")
	}
	n := 0
	for _, digit := range payload {
		if digit < '0' || digit > '9' {
			return errors.New("invalid terminal output acknowledgement")
		}
		n = n*10 + int(digit-'0')
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if n <= 0 || n > f.pending {
		return errors.New("invalid terminal output acknowledgement")
	}
	f.pending -= n
	select {
	case f.changed <- struct{}{}:
	default:
	}
	return nil
}

type flowControlledTerminalSlave struct {
	server.Slave
	flow *terminalOutputFlow
}

func (s *flowControlledTerminalSlave) Close() error {
	s.flow.cancel()
	return s.Slave.Close()
}

func (s *flowControlledTerminalSlave) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	available, err := s.flow.budget()
	if err != nil {
		return 0, err
	}
	if len(p) > available {
		p = p[:available]
	}
	n, err := s.Slave.Read(p)
	s.flow.record(n)
	return n, err
}

type flowControlledTerminalMaster struct {
	webtty.Master
	flow *terminalOutputFlow
}

func (m *flowControlledTerminalMaster) Read(p []byte) (int, error) {
	for {
		n, err := m.Master.Read(p)
		if err != nil {
			m.flow.cancel()
			return n, err
		}
		if n == 0 || p[0] != terminalOutputAck {
			return n, err
		}
		if err := m.flow.acknowledge(p[1:n]); err != nil {
			m.flow.cancel()
			return 0, err
		}
	}
}

func configureTerminalFlowControl(ctx context.Context, master webtty.Master, slave server.Slave, enabled bool) (webtty.Master, server.Slave, []webtty.Option) {
	if !enabled {
		return master, slave, nil
	}
	flowCtx, cancel := context.WithCancel(ctx)
	flow := &terminalOutputFlow{ctx: flowCtx, cancel: cancel, changed: make(chan struct{}, 1)}
	// Waiting before the PTY read keeps gotty's write mutex free: Ping and input
	// must still be handled while output waits for browser consumption.
	return &flowControlledTerminalMaster{Master: master, flow: flow},
		&flowControlledTerminalSlave{Slave: slave, flow: flow},
		[]webtty.Option{webtty.WithMasterPreferences(map[string]interface{}{
			"flowControl":       true,
			"flowControlWindow": terminalOutputWindow,
		})}
}
