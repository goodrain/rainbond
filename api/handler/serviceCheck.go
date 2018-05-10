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
	"context"
	"time"
	"fmt"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/builder/exector"
	api_db "github.com/goodrain/rainbond/api/db"
	api_model "github.com/goodrain/rainbond/api/model"
	tutil "github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/api/util"
	"github.com/twinj/uuid"
)

//ServiceCheck 应用构建源检测
func (s *ServiceAction) ServiceCheck(scs *api_model.ServiceCheckStruct) (string, string, *util.APIHandleError) {
	checkUUID := uuid.NewV4().String()
	scs.Body.CheckUUID = checkUUID
	if scs.Body.EventID == "" {
		scs.Body.EventID = tutil.NewUUID()
	}
	body, err := ffjson.Marshal(scs.Body)
	if err != nil {
		logrus.Errorf("marshal service check request body error.%s", err.Error())
		return "", "", util.CreateAPIHandleError(500, err)
	}
	bs := &api_db.BuildTaskStruct{
		TaskType: "service_check",
		TaskBody: body,
		User:     "define",
	}
	eq, errEq := api_db.BuildTaskBuild(bs)
	if errEq != nil {
		logrus.Errorf("build equeue code check error, %v", errEq)
		return "", "", util.CreateAPIHandleError(500, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	_, err = s.MQClient.Enqueue(ctx, eq)
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		return "", "", util.CreateAPIHandleError(500, err)
	}
	return checkUUID, scs.Body.EventID, nil
}

//GetServiceCheckInfo 获取应用源检测信息
func (s *ServiceAction) GetServiceCheckInfo(uuid string)(*exector.ServiceCheckResult, *util.APIHandleError) {
	k := fmt.Sprintf("/servicecheck/%s", uuid)
	var si exector.ServiceCheckResult
	ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	resp, err := s.EtcdCli.Get(ctx, k)
	cancel()
	if err != nil {
		logrus.Errorf("get etcd k %s error, %v", k, err)
		return nil, util.CreateAPIHandleError(500, err)
	}
	if resp.Count == 0 {
		return &si, nil
	}
	v := resp.Kvs[0].Value
	if err := ffjson.Unmarshal(v, &si); err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}

	if si.CheckStatus == "" {
		si.CheckStatus = "Checking"
		logrus.Debugf("checking is %v", si)
	}
	return &si, nil	
}