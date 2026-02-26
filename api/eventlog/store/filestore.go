// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/goodrain/rainbond/api/eventlog/db"
	"github.com/sirupsen/logrus"
)

// FileStore 文件存储接口
type FileStore interface {
	// Append 追加消息到文件
	Append(eventID string, message *db.EventLogMessage) error

	// ReadAll 读取所有历史消息
	ReadAll(eventID string) ([]*db.EventLogMessage, error)

	// ReadLast 读取最后N条消息
	ReadLast(eventID string, n int) ([]*db.EventLogMessage, error)

	// Delete 删除事件的所有消息
	Delete(eventID string) error

	// Clean 清理过期文件
	Clean(before time.Time) error

	// Close 关闭文件存储
	Close() error
}

// JSONLinesFileStore JSON Lines格式文件存储
// 每个EventID对应一个.jsonl文件，每行一条JSON消息
type JSONLinesFileStore struct {
	basePath  string
	fileLocks map[string]*sync.Mutex // 每个文件一个锁，保证并发写入安全
	lockMutex sync.RWMutex           // 保护fileLocks映射
	log       *logrus.Entry
}

// NewJSONLinesFileStore 创建JSON Lines文件存储
func NewJSONLinesFileStore(basePath string, log *logrus.Entry) (*JSONLinesFileStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	if log == nil {
		log = logrus.WithField("module", "filestore")
	}

	log.Infof("Initialized JSON Lines file store at %s", basePath)

	return &JSONLinesFileStore{
		basePath:  basePath,
		fileLocks: make(map[string]*sync.Mutex),
		log:       log,
	}, nil
}

// getFileLock 获取文件锁（每个文件独立锁）
func (s *JSONLinesFileStore) getFileLock(eventID string) *sync.Mutex {
	s.lockMutex.Lock()
	defer s.lockMutex.Unlock()

	if lock, ok := s.fileLocks[eventID]; ok {
		return lock
	}

	lock := &sync.Mutex{}
	s.fileLocks[eventID] = lock
	return lock
}

// Append 追加消息到文件（高性能追加写）
func (s *JSONLinesFileStore) Append(eventID string, message *db.EventLogMessage) error {
	if eventID == "" || message == nil {
		return nil
	}

	lock := s.getFileLock(eventID)
	lock.Lock()
	defer lock.Unlock()

	filePath := filepath.Join(s.basePath, eventID+".jsonl")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.log.Errorf("Failed to open file %s: %v", filePath, err)
		return err
	}
	defer file.Close()

	// 序列化消息（只保存必要字段，避免Content字段的冗余）
	data := map[string]interface{}{
		"event_id": message.EventID,
		"step":     message.Step,
		"status":   message.Status,
		"message":  message.Message,
		"level":    message.Level,
		"time":     message.Time,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		s.log.Errorf("Failed to marshal message: %v", err)
		return err
	}

	// 写入一行JSON（JSON Lines格式）
	if _, err := file.Write(append(jsonData, '\n')); err != nil {
		s.log.Errorf("Failed to write to file %s: %v", filePath, err)
		return err
	}

	return nil
}

// ReadAll 读取所有历史消息
func (s *JSONLinesFileStore) ReadAll(eventID string) ([]*db.EventLogMessage, error) {
	filePath := filepath.Join(s.basePath, eventID+".jsonl")
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 文件不存在，返回空数组
		}
		s.log.Errorf("Failed to open file %s: %v", filePath, err)
		return nil, err
	}
	defer file.Close()

	var messages []*db.EventLogMessage
	scanner := bufio.NewScanner(file)

	// 设置更大的缓冲区，避免单行过长
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		var msg db.EventLogMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			s.log.Debugf("Skip corrupted line in %s: %v", filePath, err)
			continue // 跳过损坏的行
		}
		messages = append(messages, &msg)
	}

	if err := scanner.Err(); err != nil {
		s.log.Errorf("Error reading file %s: %v", filePath, err)
		return messages, err
	}

	return messages, nil
}

// ReadLast 读取最后N条消息（用于限制历史推送量）
func (s *JSONLinesFileStore) ReadLast(eventID string, n int) ([]*db.EventLogMessage, error) {
	all, err := s.ReadAll(eventID)
	if err != nil {
		return nil, err
	}

	if len(all) <= n {
		return all, nil
	}

	return all[len(all)-n:], nil
}

// Delete 删除事件文件
func (s *JSONLinesFileStore) Delete(eventID string) error {
	s.lockMutex.Lock()
	delete(s.fileLocks, eventID)
	s.lockMutex.Unlock()

	filePath := filepath.Join(s.basePath, eventID+".jsonl")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		s.log.Errorf("Failed to delete file %s: %v", filePath, err)
		return err
	}

	s.log.Debugf("Deleted event log file: %s", filePath)
	return nil
}

// Clean 清理过期文件（用于定期清理）
func (s *JSONLinesFileStore) Clean(before time.Time) error {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		s.log.Errorf("Failed to read directory %s: %v", s.basePath, err)
		return err
	}

	cleaned := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(before) {
			filePath := filepath.Join(s.basePath, entry.Name())
			if err := os.Remove(filePath); err != nil {
				s.log.Errorf("Failed to remove old file %s: %v", filePath, err)
			} else {
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		s.log.Infof("Cleaned %d old event log files", cleaned)
	}

	return nil
}

// Close 关闭文件存储（清理资源）
func (s *JSONLinesFileStore) Close() error {
	s.lockMutex.Lock()
	defer s.lockMutex.Unlock()

	// 清空锁映射
	s.fileLocks = make(map[string]*sync.Mutex)
	s.log.Info("File store closed")

	return nil
}
