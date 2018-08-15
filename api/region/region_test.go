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

package region

import (
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	utilhttp "github.com/goodrain/rainbond/util/http"
)

func TestListTenant(t *testing.T) {
	region, _ := NewRegion(APIConf{
		Endpoints: []string{"http://kubeapi.goodrain.me:8888"},
	})
	tenants, err := region.Tenants("").List()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", tenants)
}

func TestListServices(t *testing.T) {
	region, _ := NewRegion(APIConf{
		Endpoints: []string{"http://kubeapi.goodrain.me:8888"},
	})
	services, err := region.Tenants("n93lkp7t").Services("").List()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range services {
		t.Logf("%+v", s)
	}
}

func TestDoRequest(t *testing.T) {
	region, _ := NewRegion(APIConf{
		Endpoints: []string{"http://kubeapi.goodrain.me:8888"},
	})
	var decode utilhttp.ResponseBody
	var tenants []*dbmodel.Tenants
	decode.List = &tenants
	code, err := region.DoRequest("/v2/tenants", "GET", nil, &decode)
	if err != nil {
		t.Fatal(err, code)
	}
	t.Logf("%+v", tenants)
}

func TestListNodes(t *testing.T) {
	region, _ := NewRegion(APIConf{
		Endpoints: []string{"http://kubeapi.goodrain.me:8888"},
	})
	services, err := region.Nodes().List()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range services {
		t.Logf("%+v", s)
	}
}

func TestGetNodes(t *testing.T) {
	region, _ := NewRegion(APIConf{
		Endpoints: []string{"http://kubeapi.goodrain.me:8888"},
	})
	node, err := region.Nodes().Get("a134eab8-3d42-40f5-84a5-fcf2b7a44b31")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", node)
}

func TestGetTenantsBySSL(t *testing.T) {
	region, _ := NewRegion(APIConf{
		Endpoints: []string{"https://127.0.0.1:8443"},
		Cacert:    "/Users/qingguo/gopath/src/github.com/goodrain/rainbond/test/ssl/ca.pem",
		Cert:      "/Users/qingguo/gopath/src/github.com/goodrain/rainbond/test/ssl/client.pem",
		CertKey:   "/Users/qingguo/gopath/src/github.com/goodrain/rainbond/test/ssl/client.key.pem",
	})
	tenants, err := region.Tenants("").List()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", tenants)
}
