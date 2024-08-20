// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package log

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"os"
	"strconv"
)

type CustomJSONFormatter struct{}

// Format 实现 logrus.Formatter 接口
func (f *CustomJSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	logEntry := struct {
		Level   string `json:"level"`
		Time    string `json:"time"`
		Caller  string `json:"caller"`
		Message string `json:"message"`
	}{
		Level:   entry.Level.String(),
		Time:    entry.Time.Format("2006-01-02 15:04:05.000"),
		Caller:  entry.Caller.File + ":" + strconv.Itoa(entry.Caller.Line),
		Message: entry.Message,
	}

	serialized, err := json.Marshal(logEntry)
	if err != nil {
		return nil, err
	}

	serialized = append(serialized, '\n')
	return serialized, nil
}

// 初始化logrus日志输出格式
func InitLogrus() {
	logrus.SetOutput(os.Stdout)
	logrus.SetReportCaller(true)
	logrus.SetFormatter(new(CustomJSONFormatter))
}
