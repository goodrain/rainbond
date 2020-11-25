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

package testlog

import (
	"bytes"
	"fmt"

	"github.com/goodrain/rainbond/node/nodem/logger"
	"github.com/sirupsen/logrus"
)

// Name is the name of the file that the jsonlogger logs to.
const Name = "test"

// TestLogger is Logger implementation for test
type TestLogger struct {
	buf *bytes.Buffer // json-encoded extra attributes
}

func init() {
	if err := logger.RegisterLogDriver(Name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(Name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates new JSONFileLogger which writes to filename passed in
// on given context.
func New(info logger.Info) (logger.Logger, error) {
	logrus.Debugf("create logger driver for %s", info.ContainerName)
	return &TestLogger{
		buf: bytes.NewBuffer(nil),
	}, nil
}

// Log converts logger.Message to jsonlog.JSONLog and serializes it to file.
func (l *TestLogger) Log(msg *logger.Message) error {
	fmt.Println(string(msg.Line))
	return nil
}

// ValidateLogOpt looks for json specific log options max-file & max-size.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "max-file":
		case "max-size":
		case "labels":
		case "env":
		default:
			return fmt.Errorf("unknown log opt '%s' for json-file log driver", key)
		}
	}
	return nil
}

// Close closes underlying file and signals all readers to stop.
func (l *TestLogger) Close() error {
	return nil
}

// Name returns name of this logger.
func (l *TestLogger) Name() string {
	return Name
}
