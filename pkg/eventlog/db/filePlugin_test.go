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

import "testing"

func TestGetServiceAliasID(t *testing.T) {
	t.Log(GetServiceAliasID("qwertyuiopasdfghjkl"))
}

func TestFileSaveMessage(t *testing.T) {

	f := filePlugin{
		homePath: "/Users/qingguo/",
	}
	m := &EventLogMessage{EventID: "qwertyuiopasdfghjkl"}
	m.Content = []byte("do you under stand")
	mes := []*EventLogMessage{m}
	for i := 0; i < 100; i++ {
		m := &EventLogMessage{EventID: "qwertyuiopasdfghjkl"}
		m.Content = []byte("do you under stand")
		mes = append(mes, m)
	}
	err := f.SaveMessage(mes)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMvLogFile(t *testing.T) {
	MvLogFile("/Users/qingguo/7b3d5546bd54152d/stdout.log.gz", "/Users/qingguo/7b3d5546bd54152d/stdout.log")
}
