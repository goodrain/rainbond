package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/pkg/component/storage"
)

func setupEventFilePluginTestStorage(t *testing.T) {
	t.Helper()
	storage.New()
	storage.Default().StorageCli = &storage.LocalStorage{}
}

func writeEventJSONLines(t *testing.T, homePath, eventID string, lines []string) {
	t.Helper()
	dir := filepath.Join(homePath, "eventlog")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(dir, eventID+".jsonl")
	content := []byte{}
	for _, line := range lines {
		content = append(content, []byte(line+"\n")...)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestEventFilePluginGetMessagesFallsBackToJSONLines(t *testing.T) {
	setupEventFilePluginTestStorage(t)

	tmpDir := t.TempDir()
	writeEventJSONLines(t, tmpDir, "event-1", []string{
		`{"event_id":"event-1","step":"build-exector","status":"info","message":"Starting CNB build","level":"info","time":"2026-05-06T17:53:45+08:00"}`,
		`{"event_id":"event-1","step":"build-exector","status":"failure","message":"CNB ERROR 401","level":"debug","time":"2026-05-06T17:53:50+08:00"}`,
	})

	plugin := NewEventFilePlugin(tmpDir)
	result, err := plugin.GetMessages("event-1", "debug", 0)
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}

	messages, ok := result.(MessageDataList)
	if !ok {
		t.Fatalf("GetMessages() type = %T, want MessageDataList", result)
	}
	if len(messages) != 2 {
		t.Fatalf("GetMessages() len = %d, want 2", len(messages))
	}
	if messages[1].Message != "CNB ERROR 401" {
		t.Fatalf("GetMessages() last message = %q, want CNB ERROR 401", messages[1].Message)
	}
}

func TestEventFilePluginGetMessagesFiltersJSONLinesByLevel(t *testing.T) {
	setupEventFilePluginTestStorage(t)

	tmpDir := t.TempDir()
	writeEventJSONLines(t, tmpDir, "event-2", []string{
		`{"event_id":"event-2","step":"build-exector","status":"info","message":"create CNB build job success","level":"info","time":"2026-05-06T17:53:45+08:00"}`,
		`{"event_id":"event-2","step":"build-exector","status":"failure","message":"CNB ERROR 401","level":"debug","time":"2026-05-06T17:53:50+08:00"}`,
		`{"event_id":"event-2","step":"last","status":"failure","message":"编译失败，请查看构建日志","level":"error","time":"2026-05-06T17:53:55+08:00"}`,
	})

	plugin := NewEventFilePlugin(tmpDir)
	result, err := plugin.GetMessages("event-2", "info", 0)
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}

	messages := result.(MessageDataList)
	if len(messages) != 2 {
		t.Fatalf("GetMessages() len = %d, want 2", len(messages))
	}
	if messages[0].Message != "create CNB build job success" {
		t.Fatalf("GetMessages() first message = %q", messages[0].Message)
	}
	if messages[1].Message != "编译失败，请查看构建日志" {
		t.Fatalf("GetMessages() second message = %q", messages[1].Message)
	}
}
