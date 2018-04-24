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
	"github.com/goodrain/rainbond/eventlog/conf"
	"testing"

	"github.com/Sirupsen/logrus"
	_ "github.com/go-sql-driver/mysql"
)

func TestSaveCockroachdbMessage(t *testing.T) {
	db, err := NewManager(conf.DBConf{
		Type:        "cockroachdb",
		URL:         "postgresql://root@localhost:26257/region?sslmode=disable",
		PoolSize:    2,
		PoolMaxSize: 3,
	}, logrus.WithField("modole", "dbtest"))
	if err != nil {
		t.Fatal(err)
	}
	err = db.SaveMessage([]*EventLogMessage{&EventLogMessage{
		EventID: "12123123",
		Message: "hello",
		Time:    "2016-10-10T18:00:00",
		Level:   "info",
		Content: []byte("hello"),
	}})
	if err != nil {
		t.Fatal(err)
	}
}
