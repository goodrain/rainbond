// RAINBOND, Application Management Platform
// Copyright (C) 2021-2024 Goodrain Co., Ltd.

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

package component

import (
	"context"
	"github.com/goodrain/rainbond/api/controller"
	"github.com/goodrain/rainbond/api/db"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/server"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/mq/mqcomponent/grpcserver"
	"github.com/goodrain/rainbond/mq/mqcomponent/metrics"
	"github.com/goodrain/rainbond/mq/mqcomponent/mqclient"
	"github.com/goodrain/rainbond/pkg/component/es"
	"github.com/goodrain/rainbond/pkg/component/grpc"
	"github.com/goodrain/rainbond/pkg/component/hubregistry"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/component/mq"
	"github.com/goodrain/rainbond/pkg/component/prom"
	"github.com/goodrain/rainbond/pkg/rainbond"
	"github.com/sirupsen/logrus"
	"time"
)

// Database -
func Database() rainbond.Component {
	return db.New()
}

// K8sClient -
func K8sClient() rainbond.Component {
	return k8s.New()
}

// HubRegistry -
func HubRegistry() rainbond.Component {
	return hubregistry.New()
}

// MQ -
func MQ() rainbond.Component {
	return mq.New()
}

// Prometheus -
func Prometheus() rainbond.Component {
	return prom.New()
}

// Grpc -
func Grpc() rainbond.Component {
	return grpc.New()
}

// Event -
func Event() rainbond.FuncComponent {
	logrus.Infof("init event...")
	return func(ctx context.Context, cfg *configs.Config) error {
		var tryTime time.Duration
		var err error
		for tryTime < 4 {
			tryTime++
			if err = event.NewManager(event.EventConfig{
				EventLogServers: cfg.APIConfig.EventLogServers,
			}); err != nil {
				logrus.Errorf("get event manager failed, try time is %v,%s", tryTime, err.Error())
				time.Sleep((5 + tryTime*10) * time.Second)
			} else {
				break
			}
		}
		if err != nil {
			logrus.Errorf("get event manager failed. %v", err.Error())
			return err
		}
		logrus.Info("init event manager success")
		return nil
	}
}

// Handler -
func Handler() rainbond.FuncComponent {
	return func(ctx context.Context, cfg *configs.Config) error {
		return handler.InitHandle(cfg.APIConfig)
	}
}

// Router -
func Router() rainbond.FuncComponent {
	return func(ctx context.Context, cfg *configs.Config) error {
		if err := controller.CreateV2RouterManager(cfg.APIConfig, grpc.Default().StatusClient); err != nil {
			logrus.Errorf("create v2 route manager error, %v", err)
		}
		// 启动api
		apiManager := server.NewManager(cfg.APIConfig)
		if err := apiManager.Start(); err != nil {
			return err
		}
		logrus.Info("api router is running...")
		return nil
	}
}

// Proxy -
func Proxy() rainbond.FuncComponent {
	return func(ctx context.Context, cfg *configs.Config) error {
		handler.InitProxy(cfg.APIConfig)
		return nil
	}
}

// MQHealthServer -
func MQHealthServer() rainbond.ComponentCancel {
	return metrics.New()
}

// MQGrpcServer -
func MQGrpcServer() rainbond.ComponentCancel {
	return grpcserver.New()
}

// MQClient -
func MQClient() rainbond.Component {
	return mqclient.New()
}

// ES -
func ES() rainbond.Component {
	return es.New()
}
