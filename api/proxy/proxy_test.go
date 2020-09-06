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

package proxy

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestHttpProxy(t *testing.T) {
	proxy := CreateProxy("prometheus", "http", []string{"http://106.14.145.76:9999"})

	query := fmt.Sprintf(`sum(app_resource_appfs{tenant_id=~"%s"}) by(tenant_id)`, strings.Join([]string{"824b2e9dcc4d461a852ddea20369d377"}, "|"))
	query = strings.Replace(query, " ", "%20", -1)
	fmt.Printf("http://127.0.0.1:9999/api/v1/query?query=%s", query)
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:9999/api/v1/query?query=%s", query), nil)
	if err != nil {
		logrus.Error("create request prometheus api error ", err.Error())
		return
	}
	result, err := proxy.Do(req)
	if err != nil {
		logrus.Error("do proxy request prometheus api error ", err.Error())
		return
	}
	if result.Body != nil {
		defer result.Body.Close()
		if result.StatusCode != 200 {
			fmt.Println(result.StatusCode)
		}
		// var qres queryResult
		// err = json.NewDecoder(result.Body).Decode(&qres)
		// fmt.Println(qres)
		B, _ := ioutil.ReadAll(result.Body)
		fmt.Println(string(B))
	}
}
