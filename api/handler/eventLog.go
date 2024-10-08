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

package handler

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"github.com/goodrain/rainbond/config/configs"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/goodrain/rainbond/api/model"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	eventdb "github.com/goodrain/rainbond/eventlog/db"
	"github.com/goodrain/rainbond/util/constants"
)

// LogAction  log action struct
type LogAction struct {
	eventdb *eventdb.EventFilePlugin
}

// CreateLogManager get log manager
func CreateLogManager() *LogAction {
	config := configs.Default()
	return &LogAction{
		eventdb: &eventdb.EventFilePlugin{
			HomePath: config.LogConfig.LogPath,
		},
	}
}

// GetEvents get target logs
func (l *LogAction) GetEvents(target, targetID string, page, size int) ([]*dbmodel.ServiceEvent, int, error) {
	if target == "tenant" {
		return db.GetManager().ServiceEventDao().GetEventsByTenantID(targetID, (page-1)*size, size)
	}
	return db.GetManager().ServiceEventDao().GetEventsByTarget(target, targetID, (page-1)*size, size)
}

// GetMyTeamsEvents get my team events
func (l *LogAction) GetMyTeamsEvents(target string, tenantIDs []string, page, size int) ([]*dbmodel.EventAndBuild, error) {
	if target == "tenant" {
		return db.GetManager().ServiceEventDao().GetEventsByTenantIDs(tenantIDs, (page-1)*size, size)
	}
	return nil, nil
}

// GetLogList get log list
func (l *LogAction) GetLogList(serviceAlias string) ([]*model.HistoryLogFile, error) {
	logDIR := path.Join(constants.GrdataLogPath, serviceAlias)
	_, err := os.Stat(logDIR)
	if os.IsNotExist(err) {
		return nil, err
	}
	fileList, err := ioutil.ReadDir(logDIR)
	if err != nil {
		return nil, err
	}

	var logFiles []*model.HistoryLogFile
	for _, file := range fileList {
		logfile := &model.HistoryLogFile{
			Filename:     file.Name(),
			RelativePath: path.Join("logs", serviceAlias, file.Name()),
		}
		logFiles = append(logFiles, logfile)
	}
	return logFiles, nil
}

// GetLogFile GetLogFile
func (l *LogAction) GetLogFile(serviceAlias, fileName string) (string, string, error) {
	logPath := path.Join(constants.GrdataLogPath, serviceAlias)
	fullPath := path.Join(logPath, fileName)
	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return "", "", err
	}
	return logPath, fullPath, err
}

// GetLogInstance get log web socket instance
func (l *LogAction) GetLogInstance(serviceID string) (string, error) {
	k := fmt.Sprintf("/event/dockerloginstacne/%s", serviceID)
	res, err := db.GetManager().KeyValueDao().Get(k)
	if err != nil || res == nil {
		return "", err
	}
	return res.V, nil
}

// GetLevelLog get event log
func (l *LogAction) GetLevelLog(eventID string, level string) (*apimodel.DataLog, error) {
	re, err := l.eventdb.GetMessages(eventID, level, 0)
	if err != nil {
		return nil, err
	}
	if re != nil {
		messageList, ok := re.(eventdb.MessageDataList)
		if ok {
			return &apimodel.DataLog{
				Status: "success",
				Data:   messageList,
			}, nil
		}
	}
	return &apimodel.DataLog{
		Status: "success",
		Data:   nil,
	}, nil
}

// Decompress zlib解码
func decompress(zb []byte) ([]byte, error) {
	b := bytes.NewReader(zb)
	var out bytes.Buffer
	r, err := zlib.NewReader(b)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(&out, r); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func checkLevel(level, info string) bool {
	switch level {
	case "error":
		if info == "error" {
			return true
		}
		return false
	case "info":
		if info == "info" || info == "error" {
			return true
		}
		return false
	case "debug":
		if info == "info" || info == "error" || info == "debug" {
			return true
		}
		return false
	default:
		if info == "info" || info == "error" {
			return true
		}
		return false
	}
}

func uncompress(source []byte) (re []byte, err error) {
	r, err := zlib.NewReader(bytes.NewReader(source))
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	io.Copy(&buffer, r)
	r.Close()
	return buffer.Bytes(), nil
}

func bubSort(d []apimodel.MessageData) []apimodel.MessageData {
	for i := 0; i < len(d); i++ {
		for j := i + 1; j < len(d); j++ {
			if d[i].Unixtime > d[j].Unixtime {
				temp := d[i]
				d[i] = d[j]
				d[j] = temp
			}
		}
	}
	return d
}
