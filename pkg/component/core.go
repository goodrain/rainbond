package component

import (
	"context"
	"github.com/goodrain/rainbond/api/controller"
	"github.com/goodrain/rainbond/api/db"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/server"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/pkg/component/etcd"
	"github.com/goodrain/rainbond/pkg/component/grpc"
	"github.com/goodrain/rainbond/pkg/component/hubregistry"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/rainbond"
	etcdutil "github.com/goodrain/rainbond/util/etcd"
	"github.com/sirupsen/logrus"
	"time"
)

// Database -
func Database() rainbond.Component {
	return db.Database()
}

// K8sClient -
func K8sClient() rainbond.Component {
	return k8s.K8sClient()
}

// HubRegistry -
func HubRegistry() rainbond.Component {
	return hubregistry.HubRegistry()
}

// Etcd -
func Etcd() rainbond.Component {
	return etcd.Etcd()
}

// Grpc -
func Grpc() rainbond.Component {
	return grpc.Grpc()
}

// Event -
func Event() rainbond.FuncComponent {
	logrus.Infof("init event...")
	return func(ctx context.Context, cfg *configs.Config) error {
		var tryTime time.Duration
		var err error
		etcdClientArgs := &etcdutil.ClientArgs{
			Endpoints: cfg.APIConfig.EtcdEndpoint,
			CaFile:    cfg.APIConfig.EtcdCaFile,
			CertFile:  cfg.APIConfig.EtcdCertFile,
			KeyFile:   cfg.APIConfig.EtcdKeyFile,
		}
		for tryTime < 4 {
			tryTime++
			if err = event.NewManager(event.EventConfig{
				EventLogServers: cfg.APIConfig.EventLogServers,
				DiscoverArgs:    etcdClientArgs,
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

func Router() rainbond.FuncComponent {
	return func(ctx context.Context, cfg *configs.Config) error {
		if err := controller.CreateV2RouterManager(cfg.APIConfig, grpc.Default().StatusClient); err != nil {
			logrus.Errorf("create v2 route manager error, %v", err)
		}
		// 启动api
		apiManager := server.NewManager(cfg.APIConfig, etcd.Default().EtcdClient)
		if err := apiManager.Start(); err != nil {
			return err
		}
		//defer apiManager.Stop()
		logrus.Info("api router is running...")
		return nil
	}
}

func Proxy() rainbond.FuncComponent {
	return func(ctx context.Context, cfg *configs.Config) error {
		handler.InitProxy(cfg.APIConfig)
		return nil
	}
}
