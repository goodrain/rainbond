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
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/db"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/cmd/api/option"
	cdb "github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/util"
	etcdutil "github.com/goodrain/rainbond/util/etcd"
)

var gm *GatewayAction

func init() {
	conf := option.Config{
		DBType:           "mysql",
		DBConnectionInfo: "pohF4b:EiW6Eipu@tcp(192.168.56.101:3306)/region",
	}
	//创建db manager
	if err := db.CreateDBManager(conf); err != nil {
		fmt.Printf("create db manager error, %v", err)
	}
	cli, err := client.NewMqClient(&etcdutil.ClientArgs{
		Endpoints: []string{""},
	}, "192.168.56.101:6300")
	if err != nil {
		fmt.Printf("create mq client error, %v", err)
	}
	gm = CreateGatewayManager(cdb.GetManager(), cli, nil)
}
func TestSelectAvailablePort(t *testing.T) {
	t.Log(selectAvailablePort([]int{9000}))         // less than minport
	t.Log(selectAvailablePort([]int{10000}))        // equal to minport
	t.Log(selectAvailablePort([]int{10003, 10001})) // more than minport and less than maxport
	t.Log(selectAvailablePort([]int{65535}))        // equal to maxport
	t.Log(selectAvailablePort([]int{10000, 65536})) // more than maxport
}

func TestAddHTTPRule(t *testing.T) {
	for i := 200; i < 500; i++ {
		domain := fmt.Sprintf("5000-%d.gr5d6478.aq0g1f8i.4f3597.grapps.cn", i)
		err := gm.AddHTTPRule(&apimodel.AddHTTPRuleStruct{
			HTTPRuleID:    util.NewUUID(),
			ServiceID:     "68f1b4f28d49baeb68a06e1c5f5d6478",
			ContainerPort: 5000,
			Domain:        domain,
		})
		if err != nil {
			t.Fatal(err)
		}
		logrus.Infof("add domain %s", domain)
		waitReady(domain)
		time.Sleep(time.Second * 2)
	}
}
func TestWaitReady(t *testing.T) {
	waitReady("5000-1.gr5d6478.aq0g1f8i.4f3597.grapps.cn")
}
func waitReady(domain string) bool {
	start := time.Now()
	for {
		reqAddres := "http://192.168.56.101"
		if strings.Contains(domain, "192.168.56.101") {
			reqAddres = "http://" + domain
		}
		req, _ := http.NewRequest("GET", reqAddres, nil)
		req.Host = domain
		res, _ := http.DefaultClient.Do(req)
		if res != nil && res.StatusCode == 200 {
			if res.Body != nil {
				body, _ := ioutil.ReadAll(res.Body)
				res.Body.Close()
				if strings.Contains(string(body), "2048") {
					logrus.Infof("%s is ready take %s", domain, time.Now().Sub(start))
					return true
				}
			}
		}
		time.Sleep(time.Millisecond * 500)
		continue
	}
}
func TestAddTCPRule(t *testing.T) {
	for i := 1; i < 200; i++ {
		address := fmt.Sprintf("192.168.56.101:%d", 10000+i)
		gm.AddTCPRule(&apimodel.AddTCPRuleStruct{
			TCPRuleID:     util.NewUUID(),
			ServiceID:     "68f1b4f28d49baeb68a06e1c5f5d6478",
			ContainerPort: 5000,
			IP:            "192.168.56.101",
			Port:          10000 + i,
		})
		logrus.Infof("add tcp listen %s", address)
		waitReady(address)
		time.Sleep(time.Second * 2)
	}
}
func TestDeleteHTTPRule(t *testing.T) {
	gm.DeleteHTTPRule(&apimodel.DeleteHTTPRuleStruct{HTTPRuleID: ""})
}
