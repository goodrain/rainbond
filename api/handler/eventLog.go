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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/goodrain/rainbond/util"

	api_model "github.com/goodrain/rainbond/api/model"

	"github.com/Sirupsen/logrus"

	"github.com/coreos/etcd/client"

	"os/exec"

	eventdb "github.com/goodrain/rainbond/eventlog/db"
)

//LogAction  log action struct
type LogAction struct {
	EtcdEndpoints []string
	eventdb       *eventdb.EventFilePlugin
}

//CreateLogManager get log manager
func CreateLogManager(etcdEndpoint []string) *LogAction {
	return &LogAction{
		EtcdEndpoints: etcdEndpoint,
		eventdb: &eventdb.EventFilePlugin{
			HomePath: "/grdata/logs/",
		},
	}
}

//GetLogList get log list
func (l *LogAction) GetLogList(serviceAlias string) ([]string, error) {
	downLoadDIR := "/grdata/downloads"
	urlPath := fmt.Sprintf("/log/%s", serviceAlias)
	logDIR := fmt.Sprintf("%s%s", downLoadDIR, urlPath)
	_, err := os.Stat(logDIR)
	if os.IsNotExist(err) {
		return nil, err
	}
	fileList, err := ioutil.ReadDir(logDIR)
	if err != nil {
		return nil, err
	}
	var logList []string
	if len(fileList) == 0 {
		return logList, nil
	}

	for _, file := range fileList {
		filePath := fmt.Sprintf("/logs/%s/%s", serviceAlias, file.Name())
		logrus.Debugf("filepath is %s", file.Name())
		logList = append(logList, filePath)
	}
	return logList, nil
}

//GetLogFile GetLogFile
func (l *LogAction) GetLogFile(serviceAlias, fileName string) (string, string, error) {
	downLoadDIR := "/grdata/downloads"
	urlPath := fmt.Sprintf("/log/%s", serviceAlias)
	fullPath := fmt.Sprintf("%s%s/%s", downLoadDIR, urlPath, fileName)
	logPath := fmt.Sprintf("%s/log/%s", downLoadDIR, serviceAlias)
	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return "", "", err
	}
	return logPath, fullPath, err
}

//GetLogInstance get log web socket instance
func (l *LogAction) GetLogInstance(serviceID string) (string, error) {
	//etcd V2
	etcdclient, err := client.New(client.Config{
		Endpoints: l.EtcdEndpoints,
	})
	if err != nil {
		return "", err
	}

	value, err := client.NewKeysAPI(etcdclient).Get(context.Background(),
		fmt.Sprintf("/event/dockerloginstacne/%s", serviceID),
		nil)
	if err != nil {
		return "", err
	}
	return value.Node.Value, nil
	/*
	   @ etcd V3 使用
	   	ctx, cancel := context.WithCancel(context.Background())
	   	defer cancel()
	   	etcdclientv3, err := clientv2.New(clientv3.Config{
	   		Endpoints: l.EtcdEndpoints,
	   	})
	   	value, err := etcdclientv3.Get(ctx, fmt.Sprintf("/event/dockerloginstacne/%s", serviceID))
	   	if err != nil {
	   		return "", err
	   	}
	   	if len(value.Kvs) == 0 {
	   		return "", errors.New("have no value")
	   	}
	   	return string(value.Kvs[0].Value), nil
	*/
}

//GetLevelLog 获取指定操作的操作日志
func (l *LogAction) GetLevelLog(eventID string, level string) (*api_model.DataLog, error) {
	messageList, err := l.eventdb.GetMessages(eventID, level)
	if err != nil {
		return nil, err
	}
	return &api_model.DataLog{
		Status: "success",
		Data:   messageList,
	}, nil
}

//GetLinesLogs GetLinesLogs
func (l *LogAction) GetLinesLogs(alias string, n int) ([]byte, error) {

	downLoadDIR := "/grdata/downloads"
	filePath := fmt.Sprintf("%s/log/%s/stdout.log", downLoadDIR, alias)
	if ok, err := util.FileExists(filePath); !ok {
		if err != nil {
			logrus.Errorf("check file exist error %s", err.Error())
		}
		return []byte(""), nil
	}
	f, err := exec.Command("tail", "-n", fmt.Sprintf("%d", n), filePath).Output()
	if err != nil {
		return nil, err
	}
	return f, nil
}

//Decompress zlib解码
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

func bubSort(d []api_model.MessageData) []api_model.MessageData {
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
