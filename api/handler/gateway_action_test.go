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

package handler

import (
	"fmt"
	"testing"

	"github.com/goodrain/rainbond/api/db"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/cmd/api/option"
	cdb "github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/util"
)

var gm *GatewayAction

func init() {
	conf := option.Config{
		DBType:           "mysql",
		DBConnectionInfo: "ohw4Ah:Raesa9th@tcp(192.168.2.151:3306)/region",
	}
	//创建db manager
	if err := db.CreateDBManager(conf); err != nil {
		fmt.Printf("create db manager error, %v", err)
	}
	gm = CreateGatewayManager(cdb.GetManager(), nil, nil)
}
func TestSelectAvailablePort(t *testing.T) {
	t.Log(selectAvailablePort([]int{9000}))         // less than minport
	t.Log(selectAvailablePort([]int{10000}))        // equal to minport
	t.Log(selectAvailablePort([]int{10003, 10001})) // more than minport and less than maxport
	t.Log(selectAvailablePort([]int{65535}))        // equal to maxport
	t.Log(selectAvailablePort([]int{10000, 65536})) // more than maxport
}

func TestAddHTTPRule(t *testing.T) {
	for i := 100; i < 500; i++ {
		domain := fmt.Sprintf("5000-%d.grf60b0a.1j8wbmlz.5a3a08.grapps.cn", i)
		_, err := gm.AddHTTPRule(&apimodel.AddHTTPRuleStruct{
			HTTPRuleID:    util.NewUUID(),
			ServiceID:     "56cd49e5e7f27b3c150900ef8bf60b0a",
			ContainerPort: 5000,
			Domain:        domain,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
