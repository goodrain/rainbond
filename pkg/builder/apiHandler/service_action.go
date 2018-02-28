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

package apiHandler

import (
	"github.com/Sirupsen/logrus"
	"github.com/pquerna/ffjson/ffjson"
	"os"
	"strings"
	"net/http"
	"fmt"
	"bytes"
	"github.com/goodrain/rainbond/pkg/worker/discover/model"
)

//UpgradeService 滚动升级
func UpgradeService(tenantName, serviceAlias string ,ru *model.RollingUpgradeTaskBody) error {
	url := fmt.Sprintf("http://127.0.0.1:8888/v2/tenants/%s/services/%s/upgrade", tenantName, serviceAlias)
	logrus.Debugf("rolling update new version: %s, url is %s", ru.NewDeployVersion, url)
	raw := struct {
		DeployVersion string `json:"deploy_version"`
		EventID  string `json:"event_id"`
	}{
		DeployVersion:ru.CurrentDeployVersion,
		EventID:ru.EventID,
	}
	rawBody, err := ffjson.Marshal(raw)
	if err != nil {
		return err
	}
	return publicRequest("post", url,rawBody)
}

func publicRequest(method, url string, body...[]byte) error {
	client := &http.Client{}
	var rawBody *bytes.Buffer
	if len(body) != 0 {
		rawBody = bytes.NewBuffer(body[0])  
	}else {
		rawBody = nil 
	}
	request, _ := http.NewRequest(strings.ToUpper(method), url, rawBody)
	token := os.Getenv("TOKEN")
	if token != "" {
		request.Header.Set("Authorization", "Token "+token)
	}
	response, _ := client.Do(request)
    if response.StatusCode == 200 {
        //body, _ := ioutil.ReadAll(response.Body)
        return nil
	}
	return fmt.Errorf("send upgrade mission error")
}