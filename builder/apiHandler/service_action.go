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

package apiHandler

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/worker/discover/model"
	"github.com/pquerna/ffjson/ffjson"
)

//UpgradeService 滚动升级
func UpgradeService(tenantName, serviceAlias string, ru *model.RollingUpgradeTaskBody) error {
	api := os.Getenv("REGION_API")
	if api == "" {
		api = "http://region.goodrain.me:8888"
	}
	url := fmt.Sprintf("%s/v2/tenants/%s/services/%s/upgrade", api, tenantName, serviceAlias)
	logrus.Debugf("rolling update new version: %s, url is %s", ru.NewDeployVersion, url)
	raw := struct {
		DeployVersion string `json:"deploy_version"`
		EventID       string `json:"event_id"`
	}{
		DeployVersion: ru.NewDeployVersion,
		EventID:       ru.EventID,
	}
	rawBody, err := ffjson.Marshal(raw)
	if err != nil {
		return err
	}
	return publicRequest("post", url, rawBody)
}

func publicRequest(method, url string, body ...[]byte) error {
	client := &http.Client{}
	var rawBody *bytes.Buffer
	if len(body) != 0 {
		rawBody = bytes.NewBuffer(body[0])
	} else {
		rawBody = nil
	}
	request, _ := http.NewRequest(strings.ToUpper(method), url, rawBody)
	token := os.Getenv("TOKEN")
	if token != "" {
		request.Header.Set("Authorization", "Token "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	if response.StatusCode == 200 {
		return nil
	}
	str := ""
	if response != nil && response.Body != nil {
		body, _ := ioutil.ReadAll(response.Body)
		str = string(body)
	}
	return fmt.Errorf("send upgrade mission error,response body:%s", str)
}
