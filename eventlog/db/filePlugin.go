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
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"time"
)

type filePlugin struct {
	homePath string
}

func (m *filePlugin) SaveMessage(events []*EventLogMessage) error {
	if len(events) == 0 {
		return nil
	}
	key := events[0].EventID
	var logfile *os.File
	apath := path.Join(m.homePath, GetServiceAliasID(key))
	_, err := os.Stat(apath)
	if err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(apath, 0755)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	logFile, err := os.Stat(path.Join(apath, "stdout.log"))
	if err != nil {
		if os.IsNotExist(err) {
			logfile, err = os.Create(path.Join(apath, "stdout.log"))
			if err != nil {
				return err
			}
			defer logfile.Close()
		} else {
			return err
		}
	} else {
		if logFile.ModTime().Day() != time.Now().Day() {
			err := MvLogFile(fmt.Sprintf("%s/%d-%d-%d.log.gz", apath, logFile.ModTime().Year(), logFile.ModTime().Month(), logFile.ModTime().Day()), path.Join(apath, "stdout.log"))
			if err != nil {
				return err
			}
		}
	}
	if logfile == nil {
		logfile, err = os.OpenFile(path.Join(apath, "stdout.log"), os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		defer logfile.Close()
	} else {
		defer logfile.Close()
	}
	var contant [][]byte
	for _, e := range events {
		contant = append(contant, e.Content)
	}
	body := bytes.Join(contant, []byte("\n"))
	body = append(body, []byte("\n")...)
	_, err = logfile.Write(body)
	return err
}

func (m *filePlugin) Close() error {
	return nil
}

//python:
//new_word = str(ord(string[10])) + string + str(ord(string[3])) + 'log' + str(ord(string[2]) / 7)
//new_id = hashlib.sha224(new_word).hexdigest()[0:16]
//
func GetServiceAliasID(ServiceID string) string {
	if len(ServiceID) > 11 {
		newWord := strconv.Itoa(int(ServiceID[10])) + ServiceID + strconv.Itoa(int(ServiceID[3])) + "log" + strconv.Itoa(int(ServiceID[2])/7)
		ha := sha256.New224()
		ha.Write([]byte(newWord))
		return fmt.Sprintf("%x", ha.Sum(nil))[0:16]
	}
	return ServiceID
}

//MvLogFile 更改文件名称，压缩
func MvLogFile(newName string, filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	reader, err := os.OpenFile(filePath, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	// 将压缩文档内容写入文件
	f, err := os.OpenFile(newName, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, reader)
	if err != nil {
		return err
	}
	err = os.Remove(filePath)
	if err != nil {
		return err
	}
	new, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer new.Close()
	return nil
}
