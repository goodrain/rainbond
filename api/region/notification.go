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
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/node/api/model"
	utilhttp "github.com/goodrain/rainbond/util/http"
)

//NotificationInterface cluster api
type NotificationInterface interface {
	GetNotification(start string, end string) ([]*model.NotificationEvent, *util.APIHandleError)
	HandleNotification(serviceName string, message string) ([]*model.NotificationEvent, *util.APIHandleError)
}

func (r *regionImpl) Notification() NotificationInterface {
	return &notification{prefix: "/v2/notificationEvent", regionImpl: *r}
}

type notification struct {
	regionImpl
	prefix string
}

func (n *notification) GetNotification(start string, end string) ([]*model.NotificationEvent, *util.APIHandleError) {
	var ne []*model.NotificationEvent
	var decode utilhttp.ResponseBody
	decode.List = &ne
	code, err := n.DoRequest(n.prefix+"?start="+start+"&"+"end="+end, "GET", nil, &decode)
	if err != nil {
		return nil, handleErrAndCode(err, code)
	}
	if code != 200 {
		logrus.Error("Return failure message ", decode.Msg)
		return nil, util.CreateAPIHandleError(code, fmt.Errorf(decode.Msg))
	}
	return ne, nil
}

func (n *notification) HandleNotification(serviceName string, message string) ([]*model.NotificationEvent, *util.APIHandleError) {
	var ne []*model.NotificationEvent
	var decode utilhttp.ResponseBody
	decode.List = &ne
	handleMessage, err := json.Marshal(map[string]string{"handle_message": message})
	body := bytes.NewBuffer(handleMessage)
	code, err := n.DoRequest(n.prefix+"/"+serviceName, "PUT", body, &decode)
	if err != nil {
		return nil, handleErrAndCode(err, code)
	}
	if code != 200 {
		logrus.Error("Return failure message ", decode.Msg)
		return nil, util.CreateAPIHandleError(code, fmt.Errorf(decode.Msg))
	}
	return ne, nil
}
