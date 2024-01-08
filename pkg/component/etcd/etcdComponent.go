package etcd

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/pkg/gogo"
	etcdutil "github.com/goodrain/rainbond/util/etcd"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/sirupsen/logrus"
	"time"
)

var defaultEtcdComponent *Component

type Component struct {
	EtcdClient   *clientv3.Client
	StatusClient *client.AppRuntimeSyncClient
}

func Etcd() *Component {
	defaultEtcdComponent = &Component{}
	return &Component{}
}

func Default() *Component {
	return defaultEtcdComponent
}

var (
	defaultDialTimeout      = 5 * time.Second
	defaultAotuSyncInterval = 10 * time.Second
)

func (e Component) Start(ctx context.Context, cfg *configs.Config) error {
	logrus.Info("start etcd client...")
	clientArgs := &etcdutil.ClientArgs{
		Endpoints: cfg.APIConfig.EtcdEndpoint,
		CaFile:    cfg.APIConfig.EtcdCaFile,
		CertFile:  cfg.APIConfig.EtcdCertFile,
		KeyFile:   cfg.APIConfig.EtcdKeyFile,
	}
	if clientArgs.DialTimeout <= 5 {
		clientArgs.DialTimeout = defaultDialTimeout
	}
	if clientArgs.AutoSyncInterval <= 30 {
		clientArgs.AutoSyncInterval = defaultAotuSyncInterval
	}

	config := clientv3.Config{
		Context:              ctx,
		Endpoints:            clientArgs.Endpoints,
		DialTimeout:          clientArgs.DialTimeout,
		DialKeepAliveTime:    time.Second * 2,
		DialKeepAliveTimeout: time.Second * 6,
		AutoSyncInterval:     clientArgs.AutoSyncInterval,
	}

	if clientArgs.CaFile != "" && clientArgs.CertFile != "" && clientArgs.KeyFile != "" {
		// create etcd client with tls
		tlsInfo := transport.TLSInfo{
			CertFile:      clientArgs.CertFile,
			KeyFile:       clientArgs.KeyFile,
			TrustedCAFile: clientArgs.CaFile,
		}
		tlsConfig, err := tlsInfo.ClientConfig()
		if err != nil {
			return err
		}
		config.TLS = tlsConfig
	}
	gogo.Go(func(ctx context.Context) error {
		var etcdClient *clientv3.Client
		var err error
		for {
			etcdClient, err = clientv3.New(config)
			if err == nil {
				logrus.Infof("etcd.v3 client is ready")
				e.EtcdClient = etcdClient
				e.StatusClient, err = client.NewClient(ctx, client.AppRuntimeSyncClientConf{
					EtcdEndpoints: clientArgs.Endpoints,
					EtcdCaFile:    clientArgs.CaFile,
					EtcdCertFile:  clientArgs.CertFile,
					EtcdKeyFile:   clientArgs.KeyFile,
					NonBlock:      cfg.APIConfig.Debug,
				}, etcdClient)
				return nil
			}
			logrus.Errorf("create etcd.v3 client failed, try time is %d,%s", 10, err.Error())
			time.Sleep(10 * time.Second)
		}
	})
	return nil
}

func (e Component) CloseHandle() {
}
