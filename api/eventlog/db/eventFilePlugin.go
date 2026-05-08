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

package db

import (
	"bufio"
	"encoding/json"
	"fmt"
	eventutil "github.com/goodrain/rainbond/api/eventlog/util"
	"github.com/goodrain/rainbond/pkg/component/storage"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
)

// EventFilePlugin EventFilePlugin
type EventFilePlugin struct {
	HomePath string
	// 用于控制并发上传的 goroutine 池
	uploadPool   chan struct{}
	uploadWg     sync.WaitGroup
	shutdownOnce sync.Once
	// 清理任务控制
	cleanupStopCh chan struct{}
	cleanupDone   chan struct{}
}

// 最大并发上传数
const maxConcurrentUploads = 10

// NewEventFilePlugin 创建 EventFilePlugin 实例
func NewEventFilePlugin(homePath string) *EventFilePlugin {
	plugin := &EventFilePlugin{
		HomePath:      homePath,
		uploadPool:    make(chan struct{}, maxConcurrentUploads),
		cleanupStopCh: make(chan struct{}),
		cleanupDone:   make(chan struct{}),
	}

	// 启动本地日志清理任务
	go plugin.startCleanupTask()

	return plugin
}

// SaveMessage save event log to file
func (m *EventFilePlugin) SaveMessage(events []*EventLogMessage) error {
	if len(events) == 0 {
		return nil
	}
	logrus.Debugf("init event file plugin save message")
	filePath := eventutil.EventLogFilePath(m.HomePath)
	if err := util.CheckAndCreateDir(filePath); err != nil {
		return err
	}
	filename := eventutil.EventLogFileName(filePath, events[0].EventID)
	writeFile, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer writeFile.Close()
	var lastTime int64
	for _, e := range events {
		if e == nil {
			continue
		}
		writeFile.Write(GetLevelFlag(e.Level))
		logtime := GetTimeUnix(e.Time)
		if logtime != 0 {
			lastTime = logtime
		}
		writeFile.Write([]byte(fmt.Sprintf("%d ", lastTime)))
		writeFile.Write([]byte(e.Message))
		writeFile.Write([]byte("\n"))
	}

	// 异步上传到存储（MinIO/S3），不阻塞日志写入
	m.asyncUploadWithRetry(filename)

	return nil
}

// asyncUploadWithRetry 异步上传文件到存储，带重试机制
func (m *EventFilePlugin) asyncUploadWithRetry(filename string) {
	m.uploadWg.Add(1)

	go func() {
		defer m.uploadWg.Done()

		// 获取上传槽位（限制并发数）
		m.uploadPool <- struct{}{}
		defer func() { <-m.uploadPool }()

		// 重试配置
		maxRetries := 3
		retryDelay := time.Second

		for attempt := 1; attempt <= maxRetries; attempt++ {
			err := storage.Default().StorageCli.UploadFileToFile(filename, filename, nil)
			if err == nil {
				logrus.Debugf("Successfully uploaded log file to storage: %s", filename)
				return
			}

			if attempt < maxRetries {
				logrus.Warnf("Failed to upload log file %s (attempt %d/%d): %v, retrying in %v...",
					filename, attempt, maxRetries, err, retryDelay)
				time.Sleep(retryDelay)
				// 指数退避
				retryDelay *= 2
			} else {
				logrus.Errorf("Failed to upload log file %s after %d attempts: %v",
					filename, maxRetries, err)
			}
		}
	}()
}

// MessageData message data 获取指定操作的操作日志
type MessageData struct {
	Message  string `json:"message"`
	Time     string `json:"time"`
	Unixtime int64  `json:"utime"`
}

// MessageDataList MessageDataList
type MessageDataList []MessageData

func (a MessageDataList) Len() int           { return len(a) }
func (a MessageDataList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a MessageDataList) Less(i, j int) bool { return a[i].Unixtime <= a[j].Unixtime }

// GetMessages GetMessages - directly reads from storage without downloading to local
func (m *EventFilePlugin) GetMessages(eventID, level string, length int) (interface{}, error) {
	if messages, err := m.getJSONLinesMessages(eventID, level, length); err != nil {
		logrus.Debugf("read jsonl event log error for %s: %v", eventID, err)
	} else if len(messages) > 0 {
		return messages, nil
	}

	var message MessageDataList

	// 构造存储路径（S3/MinIO 路径）
	s3Path := path.Join("grdata", "logs", "eventlog", eventID+".log")

	// 直接从存储读取文件流
	fileReader, err := m.readStorageOrLocalFile(s3Path, path.Join(m.HomePath, "eventlog", eventID+".log"))
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Warnf("Event log file not found in storage or local: %s", eventID)
			return message, nil
		}
		return nil, fmt.Errorf("failed to read event log from both storage and local: %w", err)
	}
	defer fileReader.Close()

	// 使用 bufio.Reader 逐行读取和解析
	reader := bufio.NewReader(fileReader)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				logrus.Error("read event log file error:", err.Error())
			}
			break
		}
		if len(line) > 2 {
			flag := line[0]
			if CheckLevel(string(flag), level) {
				info := strings.SplitN(string(line), " ", 3)
				if len(info) == 3 {
					timeunix := info[1]
					unix, _ := strconv.ParseInt(timeunix, 10, 64)
					tm := time.Unix(unix, 0)
					md := MessageData{
						Message:  info[2],
						Unixtime: unix,
						Time:     tm.Format(time.RFC3339),
					}
					message = append(message, md)
					if len(message) > length && length != 0 {
						break
					}
				}
			}
		}
	}
	return message, nil
}

func (m *EventFilePlugin) getJSONLinesMessages(eventID, level string, length int) (MessageDataList, error) {
	fileReader, err := m.readStorageOrLocalFile(
		path.Join("grdata", "logs", "eventlog", eventID+".jsonl"),
		path.Join(m.HomePath, "eventlog", eventID+".jsonl"))
	if err != nil {
		return nil, err
	}
	defer fileReader.Close()

	var messages MessageDataList
	reader := bufio.NewReader(fileReader)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return messages, err
		}
		if len(line) == 0 {
			continue
		}
		var eventMessage EventLogMessage
		if err := json.Unmarshal(line, &eventMessage); err != nil {
			logrus.Debugf("skip invalid jsonl line for event %s: %v", eventID, err)
			continue
		}
		if !checkStructuredLevel(eventMessage.Level, level) {
			continue
		}
		unix := GetTimeUnix(eventMessage.Time)
		eventTime := eventMessage.Time
		if unix != 0 {
			eventTime = time.Unix(unix, 0).Format(time.RFC3339)
		}
		messages = append(messages, MessageData{
			Message:  eventMessage.Message,
			Time:     eventTime,
			Unixtime: unix,
		})
		if len(messages) > length && length != 0 {
			break
		}
	}
	return messages, nil
}

func (m *EventFilePlugin) readStorageOrLocalFile(storagePath, localPath string) (storage.ReadCloser, error) {
	if storage.Default() != nil && storage.Default().StorageCli != nil {
		fileReader, err := storage.Default().StorageCli.ReadFile(storagePath)
		if err == nil {
			return fileReader, nil
		}
		logrus.Debugf("failed to read event log from storage, trying local path %s: %v", localPath, err)
	}
	localFile, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	return localFile, nil
}

func checkStructuredLevel(logLevel, queryLevel string) bool {
	switch logLevel {
	case "error":
		return true
	case "info":
		return queryLevel != "error"
	case "debug":
		return queryLevel == "debug"
	default:
		return queryLevel != "error"
	}
}

// CheckLevel check log level
func CheckLevel(flag, level string) bool {
	switch flag {
	case "0":
		return true
	case "1":
		if level != "error" {
			return true
		}
	case "2":
		if level == "debug" {
			return true
		}
	}
	return false
}

// GetTimeUnix get specified time unix
func GetTimeUnix(timeStr string) int64 {
	var timeLayout string
	if strings.Contains(timeStr, ".") {
		timeLayout = "2006-01-02T15:04:05"
	} else {
		timeLayout = "2006-01-02T15:04:05+08:00"
	}
	loc, _ := time.LoadLocation("Local")
	utime, err := time.ParseInLocation(timeLayout, timeStr, loc)
	if err != nil {
		logrus.Errorf("Parse log time error %s", err.Error())
		return 0
	}
	return utime.Unix()
}

// GetLevelFlag get log level flag
func GetLevelFlag(level string) []byte {
	switch level {
	case "error":
		return []byte("0 ")
	case "info":
		return []byte("1 ")
	case "debug":
		return []byte("2 ")
	default:
		return []byte("0 ")
	}
}

// startCleanupTask 启动本地日志清理任务
func (m *EventFilePlugin) startCleanupTask() {
	defer close(m.cleanupDone)

	// 从环境变量读取保留天数，默认 3 天
	retentionDays := 3
	if envDays := os.Getenv("EVENT_LOG_RETENTION_DAYS"); envDays != "" {
		if days, err := strconv.Atoi(envDays); err == nil && days > 0 {
			retentionDays = days
		}
	}

	logrus.Infof("Event log local cleanup task started, retention days: %d", retentionDays)

	// 立即执行一次清理
	m.cleanupOldLogs(retentionDays)

	// 定时清理：每天凌晨 2点
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// 计算到下一个凌晨 2点的时间
	now := time.Now()
	next2AM := time.Date(now.Year(), now.Month(), now.Day()+1, 2, 0, 0, 0, now.Location())
	if now.Hour() < 2 {
		// 如果当前还没到凌晨2点，今天就执行
		next2AM = time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
	}
	firstDelay := next2AM.Sub(now)

	// 首次延迟到凌晨2点
	firstTimer := time.NewTimer(firstDelay)
	defer firstTimer.Stop()

	select {
	case <-firstTimer.C:
		m.cleanupOldLogs(retentionDays)
	case <-m.cleanupStopCh:
		logrus.Info("Event log cleanup task stopped before first run")
		return
	}

	// 之后每24小时执行一次
	for {
		select {
		case <-ticker.C:
			m.cleanupOldLogs(retentionDays)
		case <-m.cleanupStopCh:
			logrus.Info("Event log cleanup task stopped")
			return
		}
	}
}

// cleanupOldLogs 清理过期的本地日志文件
func (m *EventFilePlugin) cleanupOldLogs(retentionDays int) {
	logDir := path.Join(m.HomePath, "eventlog")

	// 检查目录是否存在
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		logrus.Debugf("Event log directory does not exist: %s", logDir)
		return
	}

	logrus.Infof("Starting cleanup of event logs older than %d days in %s", retentionDays, logDir)

	// 计算截止时间
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	// 读取目录
	entries, err := os.ReadDir(logDir)
	if err != nil {
		logrus.Errorf("Failed to read event log directory %s: %v", logDir, err)
		return
	}

	deletedCount := 0
	deletedSize := int64(0)
	errorCount := 0

	for _, entry := range entries {
		// 只处理 .log 文件
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}

		filePath := path.Join(logDir, entry.Name())

		// 获取文件信息
		fileInfo, err := entry.Info()
		if err != nil {
			logrus.Warnf("Failed to get file info for %s: %v", filePath, err)
			errorCount++
			continue
		}

		// 检查修改时间
		if fileInfo.ModTime().Before(cutoffTime) {
			// 删除文件
			if err := os.Remove(filePath); err != nil {
				logrus.Errorf("Failed to delete old log file %s: %v", filePath, err)
				errorCount++
			} else {
				deletedCount++
				deletedSize += fileInfo.Size()
				logrus.Debugf("Deleted old log file: %s (age: %v, size: %d bytes)",
					entry.Name(),
					time.Since(fileInfo.ModTime()).Round(time.Hour),
					fileInfo.Size())
			}
		}
	}

	if deletedCount > 0 || errorCount > 0 {
		logrus.Infof("Event log cleanup completed: deleted %d files (%.2f MB), %d errors",
			deletedCount,
			float64(deletedSize)/(1024*1024),
			errorCount)
	} else {
		logrus.Debugf("Event log cleanup completed: no old files to delete")
	}
}

// Close 关闭插件，等待所有上传任务完成
func (m *EventFilePlugin) Close() error {
	m.shutdownOnce.Do(func() {
		logrus.Info("Waiting for all log file uploads to complete...")
		m.uploadWg.Wait()
		logrus.Info("All log file uploads completed")

		// 停止清理任务
		close(m.cleanupStopCh)
		<-m.cleanupDone
		logrus.Info("Event log cleanup task stopped")
	})
	return nil
}
