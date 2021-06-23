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

package option

import (
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
)

//Config config server
type Config struct {
	EtcdEndPoints           []string
	EtcdCaFile              string
	EtcdCertFile            string
	EtcdKeyFile             string
	EtcdTimeout             int
	EtcdPrefix              string
	ClusterName             string
	MysqlConnectionInfo     string
	DBType                  string
	PrometheusMetricPath    string
	EventLogServers         []string
	KubeConfig              string
	KubeAPIQPS              int
	KubeAPIBurst            int
	MaxTasks                int
	MQAPI                   string
	NodeName                string
	Listen                  string
	HostIP                  string
	ServerPort              int
	KubeClient              kubernetes.Interface
	LeaderElectionNamespace string
	LeaderElectionIdentity  string
	RBDNamespace            string
	GrdataPVCName           string
	Helm                    Helm
}

// Helm helm configuration.
type Helm struct {
	DataDir    string
	RepoFile   string
	RepoCache  string
	ChartCache string
}

//Worker  worker server
type Worker struct {
	Config
	LogLevel string
	RunMode  string //default,sync
}

//NewWorker new server
func NewWorker() *Worker {
	return &Worker{}
}

//AddFlags config
func (a *Worker) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&a.LogLevel, "log-level", "info", "the worker log level")
	fs.StringSliceVar(&a.EtcdEndPoints, "etcd-endpoints", []string{"http://127.0.0.1:2379"}, "etcd v3 cluster endpoints.")
	fs.StringVar(&a.EtcdCaFile, "etcd-ca", "", "")
	fs.StringVar(&a.EtcdCertFile, "etcd-cert", "", "")
	fs.StringVar(&a.EtcdKeyFile, "etcd-key", "", "")
	fs.IntVar(&a.EtcdTimeout, "etcd-timeout", 5, "etcd http timeout seconds")
	fs.StringVar(&a.EtcdPrefix, "etcd-prefix", "/store", "the etcd data save key prefix ")
	fs.StringVar(&a.PrometheusMetricPath, "metric", "/metrics", "prometheus metrics path")
	fs.StringVar(&a.Listen, "listen", ":6369", "prometheus listen host and port")
	fs.StringVar(&a.DBType, "db-type", "mysql", "db type mysql or etcd")
	fs.StringVar(&a.MysqlConnectionInfo, "mysql", "root:admin@tcp(127.0.0.1:3306)/region", "mysql db connection info")
	fs.StringSliceVar(&a.EventLogServers, "event-servers", []string{"127.0.0.1:6366"}, "event log server address. simple lb")
	fs.StringVar(&a.KubeConfig, "kube-config", "", "kubernetes api server config file")
	fs.IntVar(&a.KubeAPIQPS, "kube-api-qps", 50, "kube client qps")
	fs.IntVar(&a.KubeAPIBurst, "kube-api-burst", 10, "kube clint burst")
	fs.IntVar(&a.MaxTasks, "max-tasks", 50, "the max tasks for per node")
	fs.StringVar(&a.MQAPI, "mq-api", "127.0.0.1:6300", "acp_mq api")
	fs.StringVar(&a.RunMode, "run", "sync", "sync data when worker start")
	fs.StringVar(&a.NodeName, "node-name", "", "the name of this worker,it must be global unique name")
	fs.StringVar(&a.HostIP, "host-ip", "", "the ip of this worker,it must be global connected ip")
	fs.IntVar(&a.ServerPort, "server-port", 6535, "the listen port that app runtime server")
	fs.StringVar(&a.LeaderElectionNamespace, "leader-election-namespace", "rainbond", "Namespace where this attacher runs.")
	fs.StringVar(&a.LeaderElectionIdentity, "leader-election-identity", "", "Unique idenity of this attcher. Typically name of the pod where the attacher runs.")
	fs.StringVar(&a.RBDNamespace, "rbd-system-namespace", "rbd-system", "rbd components kubernetes namespace")
	fs.StringVar(&a.GrdataPVCName, "grdata-pvc-name", "rbd-cpt-grdata", "The name of grdata persistent volume claim")
	fs.StringVar(&a.Helm.DataDir, "helm-data-dir", "helm-data-dir", "The data directory of Helm.")

	if a.Helm.DataDir == "" {
		a.Helm.DataDir = "/grdata/helm"
	}
	a.Helm.RepoFile = path.Join(a.Helm.DataDir, "repo/repositories.yaml")
	a.Helm.RepoCache = path.Join(a.Helm.DataDir, "cache")
	a.Helm.ChartCache = path.Join(a.Helm.DataDir, "chart")
}

//SetLog 设置log
func (a *Worker) SetLog() {
	level, err := logrus.ParseLevel(a.LogLevel)
	if err != nil {
		fmt.Println("set log level error." + err.Error())
		return
	}
	logrus.SetLevel(level)
}

//CheckEnv 检测环境变量
func (a *Worker) CheckEnv() error {
	if err := os.Setenv("GRDATA_PVC_NAME", a.Config.GrdataPVCName); err != nil {
		return fmt.Errorf("set env 'GRDATA_PVC_NAME': %v", err)
	}
	if os.Getenv("EX_DOMAIN") == "" {
		return fmt.Errorf("please set env `EX_DOMAIN`")
	}
	return nil
}
