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

package event

import (
	"testing"
	"time"
)

func TestLogger(t *testing.T) {
	err := NewManager(EventConfig{
		EventLogServers: []string{"192.168.195.1:6366"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer GetManager().Close()
	time.Sleep(time.Second * 3)
	for i := 0; i < 500; i++ {
		GetManager().GetLogger("qwdawdasdasasfafa").Info("hello word", nil)
		GetManager().GetLogger("asdasdasdasdads").Debug("hello word", nil)
		GetManager().GetLogger("1234124124124").Error("hello word", nil)
		time.Sleep(time.Millisecond * 1)
	}
	select {}
}
