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
}

// 最大并发上传数
const maxConcurrentUploads = 10

// NewEventFilePlugin 创建 EventFilePlugin 实例
func NewEventFilePlugin(homePath string) *EventFilePlugin {
	return &EventFilePlugin{
		HomePath:   homePath,
		uploadPool: make(chan struct{}, maxConcurrentUploads),
	}
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

// GetMessages GetMessages
func (m *EventFilePlugin) GetMessages(eventID, level string, length int) (interface{}, error) {
	var message MessageDataList
	apath := path.Join(m.HomePath, "eventlog", eventID+".log")
	if ok, err := util.FileExists(apath); !ok {
		if err != nil {
			logrus.Errorf("check file exist error %s", err.Error())
		}

		// 从存储下载文件，带重试机制
		err = m.downloadWithRetry(eventID, apath)
		if err != nil {
			logrus.Errorf("download file to dir failure:%v", err)
			return message, nil
		}
	}
	eventFile, err := os.Open(apath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer eventFile.Close()
	reader := bufio.NewReader(eventFile)
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

// downloadWithRetry 从存储下载文件，带重试机制
func (m *EventFilePlugin) downloadWithRetry(eventID, localPath string) error {
	maxRetries := 3
	retryDelay := time.Second

	// 构造正确的 S3 路径:从 localPath 中提取相对于 HomePath 的路径
	// localPath 格式: /xxx/grdata/eventlog/eventID.log
	// 需要构造成: grdata/eventlog/eventID.log
	s3Path := path.Join("grdata", "eventlog", eventID+".log")

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := storage.Default().StorageCli.DownloadFileToDir(
			s3Path,
			path.Join(m.HomePath, "eventlog"),
		)
		if err == nil {
			logrus.Debugf("Successfully downloaded log file from storage: %s", eventID)
			return nil
		}

		if attempt < maxRetries {
			logrus.Warnf("Failed to download log file %s (attempt %d/%d): %v, retrying in %v...",
				eventID, attempt, maxRetries, err, retryDelay)
			time.Sleep(retryDelay)
			// 指数退避
			retryDelay *= 2
		} else {
			return fmt.Errorf("failed to download log file %s after %d attempts: %w",
				eventID, maxRetries, err)
		}
	}

	return nil
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

// Close 关闭插件，等待所有上传任务完成
func (m *EventFilePlugin) Close() error {
	m.shutdownOnce.Do(func() {
		logrus.Info("Waiting for all log file uploads to complete...")
		m.uploadWg.Wait()
		logrus.Info("All log file uploads completed")
	})
	return nil
}
