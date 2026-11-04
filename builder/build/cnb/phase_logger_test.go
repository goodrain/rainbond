package cnb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/event"
	corev1 "k8s.io/api/core/v1"
)

func readCNBMessages(t *testing.T, logs chan []byte) []map[string]string {
	t.Helper()
	var entries []map[string]string
	for len(logs) > 0 {
		var entry map[string]string
		if err := json.Unmarshal(<-logs, &entry); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	return entries
}

// capability_id: rainbond.cnb.phase-timing
func TestCNBPhaseTiming(t *testing.T) {
	for _, style := range []struct {
		name   string
		prefix string
		suffix string
	}{
		{name: "plain"},
		{name: "lifecycle cyan", prefix: "\x1b[36m", suffix: "\x1b[0m"},
		{name: "bold cyan", prefix: "\x1b[1;36m", suffix: "\x1b[0m"},
		{name: "true color", prefix: "\x1b[38:2:0:200:200m", suffix: "\x1b[m"},
	} {
		t.Run(style.name, func(t *testing.T) {
			logs := make(chan []byte, 20)
			var output bytes.Buffer
			writer := newCNBPhaseWriter(&output, event.NewLogger("test", logs))
			now := time.Unix(0, 0)
			writer.now = func() time.Time { return now }
			var input strings.Builder
			phases := []string{"ANALYZING", "DETECTING", "RESTORING", "BUILDING", "EXPORTING"}
			for i, phase := range phases {
				line := style.prefix + "===> " + phase + style.suffix + "\r\n"
				input.WriteString(line)
				if _, err := writer.Write([]byte(line)); err != nil {
					t.Fatal(err)
				}
				now = now.Add(time.Duration(i+1) * time.Second)
			}
			writer.finish()
			writer.finish()
			if output.String() != input.String() {
				t.Fatalf("original logs changed: %q", output.String())
			}
			entries := readCNBMessages(t, logs)
			if len(entries) != len(phases) {
				t.Fatalf("got %d timing records, want %d: %v", len(entries), len(phases), entries)
			}
			for i, entry := range entries {
				want := time.Duration(i+1) * time.Second
				if entry["step"] != "build-timing" || entry["phase"] != phases[i] || !strings.Contains(entry["message"], want.String()) {
					t.Errorf("unexpected timing entry: %v", entry)
				}
				if strings.Contains(entry["message"], "success") {
					t.Errorf("elapsed time must not imply build success: %v", entry)
				}
			}
		})
	}
}

// capability_id: rainbond.cnb.phase-timing
func TestCNBPhaseTimingHandlesFragmentedLogs(t *testing.T) {
	logs := make(chan []byte, 10)
	var output bytes.Buffer
	writer := newCNBPhaseWriter(&output, event.NewLogger("test", logs))
	now := time.Unix(0, 0)
	writer.now = func() time.Time { return now }
	writer.Write([]byte("===> REST"))
	writer.Write([]byte("ORING\nordinary log\n===> RESTORING\n===> UNKNOWN\n"))
	now = now.Add(7 * time.Second)
	writer.Write([]byte("===> BUILDING"))
	writer.finish()
	entries := readCNBMessages(t, logs)
	if len(entries) != 2 || entries[0]["phase"] != "RESTORING" || entries[0]["duration_ms"] != "7000" || entries[1]["phase"] != "BUILDING" {
		t.Fatalf("unexpected fragmented timing records: %v", entries)
	}
	if output.String() != "===> RESTORING\nordinary log\n===> RESTORING\n===> UNKNOWN\n===> BUILDING" {
		t.Fatalf("fragmented logs were changed: %q", output.String())
	}
}

type failingCNBWriter struct{}

func (failingCNBWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

// capability_id: rainbond.cnb.phase-timing
func TestCNBPhaseTimingPreservesWriterErrors(t *testing.T) {
	logs := make(chan []byte, 10)
	writer := newCNBPhaseWriter(failingCNBWriter{}, event.NewLogger("test", logs))
	if n, err := writer.Write([]byte("===> BUILDING\n")); n != 0 || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("writer result = (%d, %v), want (0, closed pipe)", n, err)
	}
	writer.finish()
	if len(logs) != 0 {
		t.Fatal("failed writes must not produce phase timing")
	}
}

// capability_id: rainbond.cnb.phase-timing
func TestCNBPhaseTimingSerializesConcurrentWrites(t *testing.T) {
	logs := make(chan []byte, 10)
	var output bytes.Buffer
	writer := newCNBPhaseWriter(&output, event.NewLogger("test", logs))
	var group sync.WaitGroup
	for i := 0; i < 10; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			writer.Write([]byte("build output\n"))
		}()
	}
	group.Wait()
	writer.finish()
	if strings.Count(output.String(), "build output\n") != 10 || len(logs) != 0 {
		t.Fatalf("concurrent output was lost or misclassified: %q", output.String())
	}
}

// capability_id: rainbond.cnb.phase-timing
func TestCNBJobEmitsFinalPhaseTiming(t *testing.T) {
	for _, status := range []string{"complete", "failed"} {
		t.Run(status, func(t *testing.T) {
			logs := make(chan []byte, 40)
			ctrl := &mockJobCtrl{execJobFn: func(ctx context.Context, pod *corev1.Pod, writer io.Writer, result *channels.RingChannel) error {
				io.WriteString(writer, "===> RESTORING\n===> BUILDING\n")
				go func() {
					result.In() <- status
					result.In() <- "logcomplete"
				}()
				return nil
			}}
			re := &build.Request{ServiceID: "svc", DeployVersion: "v1", RbdNamespace: "ns", Arch: "amd64", SourceDir: newNodeDir(t), Lang: code.Nodejs, BuildEnvs: map[string]string{}, Logger: event.NewLogger("test", logs)}
			var entries []map[string]string
			b := newTestBuilder(ctrl)
			b.jobCtrl = &timingCleanupJobCtrl{mockJobCtrl: ctrl, beforeCleanup: func() {
				entries = readCNBMessages(t, logs)
			}}
			err := b.runCNBBuildJob(re, "registry.test/app:v1")
			if (err != nil) != (status == "failed") {
				t.Fatalf("unexpected result for status %s: %v", status, err)
			}
			var phases []string
			for _, entry := range entries {
				if entry["step"] == "build-timing" {
					phases = append(phases, entry["phase"])
				}
			}
			if strings.Join(phases, ",") != "RESTORING,BUILDING" {
				t.Fatalf("missing final phase timing before cleanup on %s: %v", status, phases)
			}
		})
	}
}

type timingCleanupJobCtrl struct {
	*mockJobCtrl
	beforeCleanup func()
}

func (c *timingCleanupJobCtrl) DeleteJob(name string) {
	c.beforeCleanup()
	c.mockJobCtrl.DeleteJob(name)
}
