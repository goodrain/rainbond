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

package publiccloud

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db/model"
)

//ChargeSverify service Charge Sverify
func ChargeSverify(tenant *model.Tenants, quantity int, reason string) *util.APIHandleError {
	cloudAPI := os.Getenv("CLOUD_API")
	if cloudAPI == "" {
		cloudAPI = "http://api.goodrain.com"
	}
	regionName := os.Getenv("REGION_NAME")
	if regionName == "" {
		return util.CreateAPIHandleError(500, fmt.Errorf("region name must define in api by env REGION_NAME"))
	}
	reason = strings.Replace(reason, " ", "%20", -1)
	api := fmt.Sprintf("%s/openapi/console/v1/enterprises/%s/memory-apply?quantity=%d&tid=%s&reason=%s&region=%s", cloudAPI, tenant.EID, quantity, tenant.UUID, reason, regionName)
	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		logrus.Error("create request cloud api error", err.Error())
		return util.CreateAPIHandleError(400, fmt.Errorf("create request cloud api error"))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.Error("create request cloud api error", err.Error())
		return util.CreateAPIHandleError(400, fmt.Errorf("create request cloud api error"))
	}
	if res.Body != nil {
		defer res.Body.Close()
		rebody, _ := ioutil.ReadAll(res.Body)
		logrus.Debugf("read memory-apply response (%s)", string(rebody))
		var re = make(map[string]interface{})
		if err := ffjson.Unmarshal(rebody, &re); err == nil {
			if msg, ok := re["msg"]; ok {
				return util.CreateAPIHandleError(res.StatusCode, fmt.Errorf("%s", msg))
			}
		}
	}
	return util.CreateAPIHandleError(res.StatusCode, fmt.Errorf("none"))
}
