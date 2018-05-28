package etcd

import (
	"context"
	"github.com/Sirupsen/logrus"
	v3 "github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"os"
	"time"
)

const (
	PlatformVerifiedPrefix = "/rainbond/verified/"
)

var (
	client         *v3.Client
	defaultTimeout time.Duration
)

func Init(config option.Config) {
	logrus.Infof("create etcd client with endpoints: %v", config.EtcdEndPoints)

	defaultTimeout = time.Second * 3
	cli, err := v3.New(v3.Config{
		Endpoints:   config.EtcdEndPoints,
		DialTimeout: defaultTimeout,
	})
	if err != nil {
		logrus.Error(err)
		os.Exit(11)
	}

	client = cli
}

func Destroy() {
	logrus.Infof("close etcd client.")
	client.Close()
}

func Client() *v3.Client {
	return client
}

func Get(key string, opts ...v3.OpOption) (*v3.GetResponse, error) {
	etcCtx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	return client.Get(etcCtx, key, opts...)
}

func Put(key, val string, opts ...v3.OpOption) (*v3.PutResponse, error) {
	etcCtx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	return client.Put(etcCtx, key, val, opts...)
}

func Del(key string, opts ...v3.OpOption) (*v3.DeleteResponse, error) {
	etcCtx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	return client.Delete(etcCtx, key, opts...)
}
