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

package singleton

import (
	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/pkg/api/apiFunc"
	"github.com/goodrain/rainbond/pkg/api/apiRouters/version1"
	api_db "github.com/goodrain/rainbond/pkg/api/db"
	"github.com/goodrain/rainbond/pkg/mq/api/grpc/pb"

	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

var oneV1Singleton apiFunc.TenantInterfaceWithV1

//NewV1Singleton new v1 singleton
func NewV1Singleton(conf option.Config) (apiFunc.TenantInterfaceWithV1, *version1.ServiceStruct, error) {
	var s *version1.ServiceStruct
	if oneV1Singleton == nil {
		mqClient, kubeClient, err := newManager(conf)
		if err != nil {
			logrus.Errorf("V1 create manager error. %v", err)
			return nil, nil, err
		}
		s = &version1.ServiceStruct{
			V1API:      conf.V1API,
			MQClient:   mqClient,
			KubeClient: kubeClient,
		}
		oneV1Singleton = s
	}
	logrus.Debugf("has exist singleton.")
	return oneV1Singleton, s, nil
}

var oneV2TenantSingleton apiFunc.TenantInterface

func newManager(conf option.Config) (pb.TaskQueueClient, *kubernetes.Clientset, error) {
	mq := api_db.MQManager{
		Endpoint: conf.MQAPI,
	}
	mqClient, errMQ := mq.NewMQManager()
	if errMQ != nil {
		logrus.Errorf("new MQ manager failed, %v", errMQ)
		return nil, nil, errMQ
	}
	k8s := api_db.K8SManager{
		K8SConfig: conf.KubeConfig,
	}
	kubeClient, errK := k8s.NewKubeConnection()
	if errK != nil {
		logrus.Errorf("create kubeclient failed, %v", errK)
		return nil, nil, errK
	}
	return mqClient, kubeClient, nil
}
