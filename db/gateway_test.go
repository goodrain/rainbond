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

package db

import (
	dbconfig "github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"testing"
)

func TestIPPortImpl_GetIPByPort(t *testing.T) {
	if err := CreateManager(dbconfig.Config{
		DBType: "sqlite3",
	}); err != nil {
		t.Fatal(err)
	}
	tx := GetManager().Begin()
	tx.Delete(model.IPPort{})
	tx.Commit()

	ipport := &model.IPPort{
		UUID: util.NewUUID(),
		IP: "127.0.0.1",
		Port: 8888,
	}
	if err := GetManager().IPPortDao().AddModel(ipport); err != nil {
		t.Fatal(err)
	}

	ports, err := GetManager().IPPortDao().GetIPByPort(8888)
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Fatalf("Expected 1 for length of ports, but returned %d)", len(ports))
	}
}
