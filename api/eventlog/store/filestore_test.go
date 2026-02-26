// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goodrain/rainbond/api/eventlog/db"
	"github.com/sirupsen/logrus"
)

func TestJSONLinesFileStore(t *testing.T) {
	// 创建临时目录
	tmpDir := filepath.Join(os.TempDir(), "rainbond-test-eventlog")
	defer os.RemoveAll(tmpDir)

	log := logrus.WithField("test", "filestore")
	store, err := NewJSONLinesFileStore(tmpDir, log)
	if err != nil {
		t.Fatalf("Failed to create file store: %v", err)
	}
	defer store.Close()

	eventID := "test-event-123"

	t.Run("Append and Read", func(t *testing.T) {
		// 写入测试消息
		messages := []*db.EventLogMessage{
			{
				EventID: eventID,
				Step:    "step1",
				Status:  "running",
				Message: "Test message 1",
				Level:   "info",
				Time:    "2024-01-01 10:00:00",
			},
			{
				EventID: eventID,
				Step:    "step2",
				Status:  "running",
				Message: "Test message 2",
				Level:   "info",
				Time:    "2024-01-01 10:00:01",
			},
			{
				EventID: eventID,
				Step:    "step3",
				Status:  "success",
				Message: "Test message 3",
				Level:   "info",
				Time:    "2024-01-01 10:00:02",
			},
		}

		for _, msg := range messages {
			if err := store.Append(eventID, msg); err != nil {
				t.Fatalf("Failed to append message: %v", err)
			}
		}

		// 读取所有消息
		result, err := store.ReadAll(eventID)
		if err != nil {
			t.Fatalf("Failed to read messages: %v", err)
		}

		if len(result) != 3 {
			t.Fatalf("Expected 3 messages, got %d", len(result))
		}

		// 验证内容
		if result[0].Message != "Test message 1" {
			t.Errorf("Message 1 mismatch: got %s", result[0].Message)
		}
		if result[1].Status != "running" {
			t.Errorf("Status 2 mismatch: got %s", result[1].Status)
		}
		if result[2].Step != "step3" {
			t.Errorf("Step 3 mismatch: got %s", result[2].Step)
		}
	})

	t.Run("ReadLast", func(t *testing.T) {
		// 读取最后2条
		result, err := store.ReadLast(eventID, 2)
		if err != nil {
			t.Fatalf("Failed to read last messages: %v", err)
		}

		if len(result) != 2 {
			t.Fatalf("Expected 2 messages, got %d", len(result))
		}

		if result[0].Message != "Test message 2" {
			t.Errorf("Message mismatch: got %s", result[0].Message)
		}
		if result[1].Message != "Test message 3" {
			t.Errorf("Message mismatch: got %s", result[1].Message)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		// 删除文件
		if err := store.Delete(eventID); err != nil {
			t.Fatalf("Failed to delete: %v", err)
		}

		// 验证文件已删除
		result, err := store.ReadAll(eventID)
		if err != nil {
			t.Fatalf("Failed to read after delete: %v", err)
		}

		if len(result) != 0 {
			t.Errorf("Expected 0 messages after delete, got %d", len(result))
		}
	})

	t.Run("Clean", func(t *testing.T) {
		// 创建一些测试文件
		oldEvent := "old-event"
		store.Append(oldEvent, &db.EventLogMessage{
			EventID: oldEvent,
			Message: "old message",
		})

		// 等待一下
		time.Sleep(100 * time.Millisecond)

		// 清理100ms前的文件
		if err := store.Clean(time.Now().Add(-50 * time.Millisecond)); err != nil {
			t.Fatalf("Failed to clean: %v", err)
		}

		// 验证文件被清理
		result, _ := store.ReadAll(oldEvent)
		if len(result) != 0 {
			t.Errorf("Expected file to be cleaned")
		}
	})
}

func TestFileStoreConcurrency(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "rainbond-test-concurrent")
	defer os.RemoveAll(tmpDir)

	log := logrus.WithField("test", "concurrent")
	store, err := NewJSONLinesFileStore(tmpDir, log)
	if err != nil {
		t.Fatalf("Failed to create file store: %v", err)
	}
	defer store.Close()

	eventID := "concurrent-event"

	// 并发写入
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				msg := &db.EventLogMessage{
					EventID: eventID,
					Message: "Concurrent message",
					Step:    "step",
				}
				store.Append(eventID, msg)
			}
			done <- true
		}(i)
	}

	// 等待完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证消息数量
	result, err := store.ReadAll(eventID)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if len(result) != 1000 {
		t.Errorf("Expected 1000 messages, got %d", len(result))
	}
}
