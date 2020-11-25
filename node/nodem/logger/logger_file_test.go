// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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

package logger

import (
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestReadFile(t *testing.T) {
	reader, err := NewLogFile("../../../test/dockerlog/tes.log", 3, false, decodeFunc, 0640, getTailReader)
	if err != nil {
		t.Fatal(err)
	}
	watch := NewLogWatcher()
	reader.ReadLogs(ReadConfig{Follow: true, Tail: 0, Since: time.Now()}, watch)
	defer watch.ConsumerGone()
LogLoop:
	for {
		select {
		case msg, ok := <-watch.Msg:
			if !ok {
				break LogLoop
			}
			fmt.Println(string(msg.Line))
		case err := <-watch.Err:
			logrus.Errorf("Error streaming logs: %v", err)
			break LogLoop
		}
	}
}
