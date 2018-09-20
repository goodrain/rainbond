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
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/util"
)

//EventFilePlugin EventFilePlugin
type EventFilePlugin struct {
	HomePath string
}

//SaveMessage save event log to file
func (m *EventFilePlugin) SaveMessage(events []*EventLogMessage) error {
	if len(events) == 0 {
		return nil
	}
	key := events[0].EventID
	dirpath := path.Join(m.HomePath, "eventlog")
	if err := util.CheckAndCreateDir(dirpath); err != nil {
		return err
	}
	apath := path.Join(m.HomePath, "eventlog", key+".log")
	writeFile, err := os.OpenFile(apath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer writeFile.Close()
	var lastTime int64
	for _, e := range events {
		writeFile.Write(GetLevelFlag(e.Level))
		logtime := GetTimeUnix(e.Time)
		if logtime != 0 {
			lastTime = logtime
		}
		writeFile.Write([]byte(fmt.Sprintf("%d ", lastTime)))
		writeFile.Write([]byte(e.Message))
		writeFile.Write([]byte("\n"))
	}
	return nil
}

//MessageData message data 获取指定操作的操作日志
type MessageData struct {
	Message  string `json:"message"`
	Time     string `json:"time"`
	Unixtime int64  `json:"utime"`
}

//MessageDataList MessageDataList
type MessageDataList []MessageData

func (a MessageDataList) Len() int           { return len(a) }
func (a MessageDataList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a MessageDataList) Less(i, j int) bool { return a[i].Unixtime <= a[j].Unixtime }

//GetMessages GetMessages
func (m *EventFilePlugin) GetMessages(eventID, level string) (MessageDataList, error) {
	apath := path.Join(m.HomePath, "eventlog", eventID+".log")
	eventFile, err := os.Open(apath)
	if err != nil {
		return nil, err
	}
	defer eventFile.Close()
	reader := bufio.NewReader(eventFile)
	var message MessageDataList
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
				fmt.Println(info)
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
				}
			}
		}
	}
	//Multi-node eventlog is valid.
	sort.Sort(message)
	return message, nil
}

//CheckLevel check log level
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

//GetTimeUnix get specified time unix
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

//GetLevelFlag get log level flag
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

//Close Close
func (m *EventFilePlugin) Close() error {
	return nil
}
