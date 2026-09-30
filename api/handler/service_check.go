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

package handler

import (
	"encoding/json"
	"fmt"
	"strings"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/builder/exector"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/db"
	client "github.com/goodrain/rainbond/mq/client"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	tutil "github.com/goodrain/rainbond/util"
	"github.com/google/uuid"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
)

// ServiceCheck check service build source
func (s *ServiceAction) ServiceCheck(scs *apimodel.ServiceCheckStruct) (string, string, *util.APIHandleError) {
	checkUUID := uuid.New().String()
	scs.Body.CheckUUID = checkUUID
	if scs.Body.EventID == "" {
		scs.Body.EventID = tutil.NewUUID()
	}
	topic := client.BuilderTopic
	config := configs.Default()
	if tutil.StringArrayContains(config.APIConfig.EnableFeature, "windows") {
		if scs.Body.CheckOS == "windows" {
			topic = client.WindowsBuilderTopic
		}
		if scs.Body.SourceType == "docker-run" || scs.Body.SourceType == "docker-compose" {
			if maybeIsWindowsContainerImage(scs.Body.SourceBody) {
				topic = client.WindowsBuilderTopic
			}
		}
	}
	body, err := json.Marshal(scs.Body)
	if err != nil {
		return "", "", util.CreateAPIHandleError(400, fmt.Errorf("invalid check request"))
	}
	_, needs, err := guard.PackageCheckIdentity(body)
	if err != nil {
		return "", "", util.CreateAPIHandleError(400, fmt.Errorf("invalid check identity"))
	}
	if needs {
		if db.GetManager() == nil {
			return "", "", util.CreateAPIHandleError(503, fmt.Errorf("package check admission unavailable"))
		}
		if err := guard.ReserveQueuedNativeTask(db.GetManager().DB(), "service-check", checkUUID, body); err != nil {
			return "", "", util.CreateAPIHandleError(503, fmt.Errorf("package check admission unavailable"))
		}
	}
	err = s.MQClient.SendBuilderTopic(client.TaskStruct{
		TaskType: "service_check",
		TaskBody: scs.Body,
		Topic:    topic,
	})
	if err != nil {
		logrus.Errorf("enqueue service check message to mq error, %v", err)
		return "", "", util.CreateAPIHandleError(500, err)
	}
	return checkUUID, scs.Body.EventID, nil
}

var windowsKeywords = []string{"windows", "asp", "microsoft", "nanoserver"}

func maybeIsWindowsContainerImage(source string) bool {
	for _, k := range windowsKeywords {
		if strings.Contains(source, k) {
			return true
		}
	}
	return false

}

// GetServiceCheckInfo get application source detection information
func (s *ServiceAction) GetServiceCheckInfo(uuid string) (*exector.ServiceCheckResult, *util.APIHandleError) {
	k := fmt.Sprintf("/servicecheck/%s", uuid)
	var si exector.ServiceCheckResult
	resp, err := db.GetManager().KeyValueDao().Get(k)
	if err != nil || resp == nil {
		return &si, nil
	}

	if err := ffjson.Unmarshal([]byte(resp.V), &si); err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	if si.CheckStatus == "" {
		si.CheckStatus = "Checking"
		logrus.Debugf("checking is %v", si)
	}
	return &si, nil
}
