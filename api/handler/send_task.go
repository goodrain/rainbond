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
	"github.com/Sirupsen/logrus"
	api_db "github.com/goodrain/rainbond/api/db"
	"github.com/goodrain/rainbond/mq/api/grpc/client"
	"github.com/pquerna/ffjson/ffjson"
)

func sendTask(body map[string]interface{}, taskType string, mqClient *client.MQClient) error {
	bodyJ, err := ffjson.Marshal(body)
	if err != nil {
		return err
	}
	bs := &api_db.BuildTaskStruct{
		TaskType: taskType,
		TaskBody: bodyJ,
		User:     "define",
	}
	eq, errEq := api_db.BuildTaskBuild(bs)
	if errEq != nil {
		logrus.Errorf("build equeue stop request error, %v", errEq)
		return errEq
	}
	ctx, cancel := context.WithCancel(context.Background())
	reply, err := mqClient.Enqueue(ctx, eq)
	cancel()
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		return err
	}
	logrus.Debugf("Enqueue replay: %s, topics: %v, message: %s", reply.Status, reply.Topics, reply.Message)
	return nil
}
