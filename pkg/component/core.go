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
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/api/controller"
	api_db "github.com/goodrain/rainbond/api/db"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/server"
	"github.com/goodrain/rainbond/builder/api"
	"github.com/goodrain/rainbond/builder/clean"
	chaos_discover "github.com/goodrain/rainbond/builder/discover"
	"github.com/goodrain/rainbond/builder/exector"
	exec_monitor "github.com/goodrain/rainbond/builder/monitor"
	"github.com/goodrain/rainbond/config/configs"
	db "github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/mq/mqcomponent/grpcserver"
	"github.com/goodrain/rainbond/mq/mqcomponent/metrics"
	"github.com/goodrain/rainbond/mq/mqcomponent/mqclient"
	"github.com/goodrain/rainbond/pkg/component/eventlog"
	"github.com/goodrain/rainbond/pkg/component/grpc"
	"github.com/goodrain/rainbond/pkg/component/hubregistry"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/component/mq"
	"github.com/goodrain/rainbond/pkg/component/prom"
	"github.com/goodrain/rainbond/pkg/component/storage"
	"github.com/goodrain/rainbond/pkg/gogo"
	"github.com/goodrain/rainbond/pkg/rainbond"
	"github.com/goodrain/rainbond/worker/appm/componentdefinition"
	worker_controller "github.com/goodrain/rainbond/worker/appm/controller"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/discover"
	"github.com/goodrain/rainbond/worker/gc"
	"github.com/goodrain/rainbond/worker/master"
	"github.com/goodrain/rainbond/worker/monitor"
	worker_server "github.com/goodrain/rainbond/worker/server"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"net/http"
	"time"
)

// Database -
func Database() rainbond.Component {
	return api_db.New()
}

// K8sClient -
func K8sClient() rainbond.Component {
	return k8s.New()
}

// StorageClient -
func StorageClient() rainbond.Component {
	return storage.New()
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
	return func(ctx context.Context) error {
		var tryTime time.Duration
		var err error
		for tryTime < 4 {
			tryTime++
			if err = event.NewManager(event.EventConfig{
				EventLogServers: configs.Default().ServerConfig.EventLogServers,
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

// APIHandler -
func APIHandler() rainbond.FuncComponent {
	return func(ctx context.Context) error {
		return handler.InitAPIHandle()
	}
}

// APIRouter -
func APIRouter() rainbond.FuncComponent {
	logrus.Infof("init router..., eventlog socket server is %+v, entry is %+v", eventlog.Default().SocketServer, eventlog.Default().Entry)
	return func(ctx context.Context) error {
		if err := controller.CreateV2RouterManager(grpc.Default().StatusClient); err != nil {
			logrus.Errorf("create v2 route manager error, %v", err)
		}
		// 启动api
		apiManager := server.NewManager()
		if err := apiManager.Start(); err != nil {
			return err
		}
		logrus.Info("api router is running...")
		return nil
	}
}

// WorkerInit -
func WorkerInit() rainbond.FuncComponent {
	return func(ctx context.Context) error {
		errChan := make(chan error, 2)
		updateCh := channels.NewRingChannel(1024)
		componentdefinition.NewComponentDefinitionBuilder()
		cacheStore := store.NewStore(db.GetManager())
		if err := cacheStore.Start(); err != nil {
			logrus.Error("start kube cache store error", err)
			return err
		}
		go func() {
			controllerManager := worker_controller.NewManager(cacheStore)
			masterCon, err := master.NewMasterController(cacheStore)
			if err != nil {
				errChan <- err
				return
			}
			if err := masterCon.Start(); err != nil {
				errChan <- err
				return
			}
			garbageCollector := gc.NewGarbageCollector()
			taskManager := discover.NewTaskManager(cacheStore, controllerManager, garbageCollector)
			if err := taskManager.Start(); err != nil {
				errChan <- err
				return
			}
			runtimeServer := worker_server.CreaterRuntimeServer(cacheStore, updateCh)
			runtimeServer.Start(errChan)
			exporterManager := monitor.NewManager(masterCon, controllerManager)
			if err := exporterManager.Start(); err != nil {
				errChan <- err
				return
			}
			defer func() {
				controllerManager.Stop()
				masterCon.Stop()
				taskManager.Stop()
				logrus.Info("shutting down...")
			}()
			logrus.Info("worker router is running...")
			select {
			case <-ctx.Done():
				logrus.Info("context cancelled, shutting down...")
				return
			case err := <-errChan:
				logrus.Error("error occurred:", err)
				return
			}
		}()
		return nil
	}
}

// ChaosInit -
func ChaosInit() rainbond.FuncComponent {
	return func(ctx context.Context) error {
		errChan := make(chan error)
		exec, err := exector.NewManager()
		if err != nil {
			return err
		}
		if err := exec.Start(); err != nil {
			return err
		}
		//exec manage stop by discover
		go func() {
			dis := chaos_discover.NewChaosTaskManager(exec)
			if err := dis.Start(errChan); err != nil {
				errChan <- err
				return
			}
			//默认清理策略：保留最新构建成功的5份，过期镜像将会清理本地和rbd-hub
			cle, err := clean.CreateCleanManager(exec.GetImageClient())
			if err != nil {
				errChan <- err
				return
			}
			if configs.Default().ChaosConfig.CleanUp {
				if err := cle.Start(errChan); err != nil {
					errChan <- err
					return
				}

			}
			exporter := exec_monitor.NewExporter(exec)
			prometheus.MustRegister(exporter)
			defer func() {
				dis.Stop()
				cle.Stop()
				logrus.Info("shutting down...")
			}()
			select {
			case <-ctx.Done():
				logrus.Info("context cancelled, shutting down...")
				return
			case err := <-errChan:
				logrus.Error("error occurred:", err)
				return
			}
		}()
		return nil
	}
}

// ChaosRouter -
func ChaosRouter() rainbond.FuncComponent {
	return func(ctx context.Context) error {
		r := api.APIServer()
		r.Handle(configs.Default().PrometheusConfig.PrometheusMetricPath, promhttp.Handler())
		logrus.Info("builder api listen port 3228")
		_ = gogo.Go(func(ctx context.Context) error {
			return http.ListenAndServe(":3228", r)
		})
		return nil
	}
}

// Proxy -
func Proxy() rainbond.FuncComponent {
	return func(ctx context.Context) error {
		handler.InitProxy()
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

// EventLog -
func EventLog() rainbond.Component {
	return eventlog.New()
}
