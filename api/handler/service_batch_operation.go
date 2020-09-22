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
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	gclient "github.com/goodrain/rainbond/mq/client"
)

//BatchOperationHandler batch operation handler
type BatchOperationHandler struct {
	mqCli            gclient.MQClient
	operationHandler *OperationHandler
}

//BatchOperationResult batch operation result
type BatchOperationResult struct {
	BatchResult []OperationResult `json:"batche_result"`
}

//CreateBatchOperationHandler create batch operation handler
func CreateBatchOperationHandler(mqCli gclient.MQClient, operationHandler *OperationHandler) *BatchOperationHandler {
	return &BatchOperationHandler{
		mqCli:            mqCli,
		operationHandler: operationHandler,
	}
}

func setStartupSequenceConfig(configs map[string]string) map[string]string {
	if configs == nil {
		configs = make(map[string]string, 1)
	}
	configs["needStartupSequence"] = "true"
	return configs
}

func checkResourceEnough(serviceID string) error {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		logrus.Errorf("get service by id error, %v", err)
		return err
	}
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(service.TenantID)
	if err != nil {
		logrus.Errorf("get tenant by id error: %v", err)
		return err
	}

	return CheckTenantResource(tenant, service.ContainerMemory*service.Replicas)
}

//Build build
func (b *BatchOperationHandler) Build(buildInfos []model.BuildInfoRequestStruct) (re BatchOperationResult) {
	var serviceIDs []string
	for _, info := range buildInfos {
		serviceIDs = append(serviceIDs, info.ServiceID)
	}

	var retrys []model.BuildInfoRequestStruct
	for _, buildInfo := range buildInfos {
		if err := checkResourceEnough(buildInfo.ServiceID); err != nil {
			re.BatchResult = append(re.BatchResult, OperationResult{
				ServiceID:     buildInfo.ServiceID,
				Operation:     "build",
				EventID:       buildInfo.EventID,
				Status:        "failure",
				ErrMsg:        err.Error(),
				DeployVersion: "",
			})
			continue
		}
		buildInfo.Configs = setStartupSequenceConfig(buildInfo.Configs)
		buildre := b.operationHandler.Build(buildInfo)
		if buildre.Status != "success" {
			retrys = append(retrys, buildInfo)
		} else {
			re.BatchResult = append(re.BatchResult, buildre)
		}
	}
	for _, retry := range retrys {
		re.BatchResult = append(re.BatchResult, b.operationHandler.Build(retry))
	}
	return
}

//Start batch start
func (b *BatchOperationHandler) Start(startInfos []model.StartOrStopInfoRequestStruct) (re BatchOperationResult) {
	var serviceIDs []string
	for _, info := range startInfos {
		serviceIDs = append(serviceIDs, info.ServiceID)
	}

	var retrys []model.StartOrStopInfoRequestStruct
	for _, startInfo := range startInfos {
		if err := checkResourceEnough(startInfo.ServiceID); err != nil {
			re.BatchResult = append(re.BatchResult, OperationResult{
				ServiceID:     startInfo.ServiceID,
				Operation:     "start",
				EventID:       startInfo.EventID,
				Status:        "failure",
				ErrMsg:        err.Error(),
				DeployVersion: "",
			})
			continue
		}

		// startup sequence
		startInfo.Configs = setStartupSequenceConfig(startInfo.Configs)
		startre := b.operationHandler.Start(startInfo)
		if startre.Status != "success" {
			retrys = append(retrys, startInfo)
		} else {
			re.BatchResult = append(re.BatchResult, startre)
		}
	}
	for _, retry := range retrys {
		re.BatchResult = append(re.BatchResult, b.operationHandler.Start(retry))
	}
	return
}

//Stop batch stop
func (b *BatchOperationHandler) Stop(stopInfos []model.StartOrStopInfoRequestStruct) (re BatchOperationResult) {
	var retrys []model.StartOrStopInfoRequestStruct
	for _, stopInfo := range stopInfos {
		stopre := b.operationHandler.Stop(stopInfo)
		if stopre.Status != "success" {
			retrys = append(retrys, stopInfo)
		} else {
			re.BatchResult = append(re.BatchResult, stopre)
		}
	}
	for _, retry := range retrys {
		re.BatchResult = append(re.BatchResult, b.operationHandler.Stop(retry))
	}
	return
}

//Upgrade batch upgrade
func (b *BatchOperationHandler) Upgrade(upgradeInfos []model.UpgradeInfoRequestStruct) (re BatchOperationResult) {
	var serviceIDs []string
	for _, info := range upgradeInfos {
		serviceIDs = append(serviceIDs, info.ServiceID)
	}

	var retrys []model.UpgradeInfoRequestStruct
	for _, upgradeInfo := range upgradeInfos {
		if err := checkResourceEnough(upgradeInfo.ServiceID); err != nil {
			re.BatchResult = append(re.BatchResult, OperationResult{
				ServiceID:     upgradeInfo.ServiceID,
				Operation:     "upgrade",
				EventID:       upgradeInfo.EventID,
				Status:        "failure",
				ErrMsg:        err.Error(),
				DeployVersion: "",
			})
			continue
		}
		upgradeInfo.Configs = setStartupSequenceConfig(upgradeInfo.Configs)
		stopre := b.operationHandler.Upgrade(upgradeInfo)
		if stopre.Status != "success" {
			retrys = append(retrys, upgradeInfo)
		} else {
			re.BatchResult = append(re.BatchResult, stopre)
		}
	}
	for _, retry := range retrys {
		re.BatchResult = append(re.BatchResult, b.operationHandler.Upgrade(retry))
	}
	return
}
