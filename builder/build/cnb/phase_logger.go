package cnb

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/event"
)

// cnbPhaseWriter preserves build output and reports observed lifecycle phase
// durations. The final phase includes the wait for job and log completion;
// parallel app/cache export is reported as one EXPORTING phase.
type cnbPhaseWriter struct {
	mu       sync.Mutex
	writer   io.Writer
	logger   event.Logger
	now      func() time.Time
	pending  []byte
	phase    string
	started  time.Time
	finished bool
}

func newCNBPhaseWriter(writer io.Writer, logger event.Logger) *cnbPhaseWriter {
	return &cnbPhaseWriter{writer: writer, logger: logger, now: time.Now}
}

func (w *cnbPhaseWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err := w.writer.Write(data)
	if !w.finished {
		w.pending = append(w.pending, data[:n]...)
		for {
			end := bytes.IndexByte(w.pending, '\n')
			if end < 0 {
				break
			}
			w.observeLine(string(w.pending[:end]))
			w.pending = w.pending[end+1:]
		}
		if len(w.pending) == 0 {
			w.pending = nil
		}
	}
	return n, err
}

func (w *cnbPhaseWriter) observeLine(line string) {
	var phase string
	switch strings.TrimSpace(line) {
	case "===> ANALYZING":
		phase = "ANALYZING"
	case "===> DETECTING":
		phase = "DETECTING"
	case "===> RESTORING":
		phase = "RESTORING"
	case "===> BUILDING":
		phase = "BUILDING"
	case "===> EXPORTING":
		phase = "EXPORTING"
	default:
		return
	}
	if phase == w.phase {
		return
	}
	now := w.now()
	w.report(now)
	w.phase, w.started = phase, now
}

func (w *cnbPhaseWriter) report(now time.Time) {
	if w.phase == "" {
		return
	}
	elapsed := now.Sub(w.started).Round(time.Millisecond)
	w.logger.Info(fmt.Sprintf("CNB phase %s observed elapsed: %s", w.phase, elapsed),
		map[string]string{
			"step":        "build-timing",
			"phase":       w.phase,
			"duration_ms": strconv.FormatInt(elapsed.Milliseconds(), 10),
		})
}

func (w *cnbPhaseWriter) finish() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.finished {
		return
	}
	if len(w.pending) > 0 {
		w.observeLine(string(w.pending))
		w.pending = nil
	}
	w.report(w.now())
	w.finished = true
}
