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

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/spf13/pflag"
)

//Config config server
type Config struct {
	EtcdEndPoints        []string
	EtcdCaFile           string
	EtcdCertFile         string
	EtcdKeyFile          string
	EtcdTimeout          int
	EtcdPrefix           string
	ClusterName          string
	MysqlConnectionInfo  string
	DBType               string
	PrometheusMetricPath string
	EventLogServers      []string
	KubeConfig           string
	MaxTasks             int
	APIPort              int
	MQAPI                string
	DockerEndpoint       string
	HostIP               string
	CleanUp              bool
	Topic                string
	LogPath              string
	RbdNamespace         string
	RbdRepoName          string
}

//Builder  builder server
type Builder struct {
	Config
	LogLevel string
	RunMode  string //default,sync
}

//NewBuilder new server
func NewBuilder() *Builder {
	return &Builder{}
}

//AddFlags config
func (a *Builder) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&a.LogLevel, "log-level", "info", "the builder log level")
	fs.StringSliceVar(&a.EtcdEndPoints, "etcd-endpoints", []string{"http://127.0.0.1:2379"}, "etcd v3 cluster endpoints.")
	fs.StringVar(&a.EtcdCaFile, "etcd-ca", "", "")
	fs.StringVar(&a.EtcdCertFile, "etcd-cert", "", "")
	fs.StringVar(&a.EtcdKeyFile, "etcd-key", "", "")
	fs.IntVar(&a.EtcdTimeout, "etcd-timeout", 5, "etcd http timeout seconds")
	fs.StringVar(&a.EtcdPrefix, "etcd-prefix", "/store", "the etcd data save key prefix ")
	fs.StringVar(&a.PrometheusMetricPath, "metric", "/metrics", "prometheus metrics path")
	fs.StringVar(&a.DBType, "db-type", "mysql", "db type mysql or etcd")
	fs.StringVar(&a.MysqlConnectionInfo, "mysql", "root:admin@tcp(127.0.0.1:3306)/region", "mysql db connection info")
	fs.StringSliceVar(&a.EventLogServers, "event-servers", []string{"127.0.0.1:6366"}, "event log server address. simple lb")
	fs.StringVar(&a.KubeConfig, "kube-config", "", "kubernetes api server config file")
	fs.IntVar(&a.MaxTasks, "max-tasks", 0, "Maximum number of simultaneous build tasks，If set to 0, the maximum limit is twice the number of CPU cores")
	fs.IntVar(&a.APIPort, "api-port", 3228, "the port for api server")
	fs.StringVar(&a.MQAPI, "mq-api", "127.0.0.1:6300", "acp_mq api")
	fs.StringVar(&a.RunMode, "run", "sync", "sync data when worker start")
	fs.StringVar(&a.DockerEndpoint, "dockerd", "127.0.0.1:2376", "dockerd endpoint")
	fs.StringVar(&a.HostIP, "hostIP", "", "Current node Intranet IP")
	fs.BoolVar(&a.CleanUp, "clean-up", true, "Turn on build version cleanup")
	fs.StringVar(&a.Topic, "topic", "builder", "Topic in mq,you coule choose `builder` or `windows_builder`")
	fs.StringVar(&a.LogPath, "log-path", "/grdata/logs", "Where Docker log files and event log files are stored.")
	fs.StringVar(&a.RbdNamespace, "rbd-namespace", "rbd-system", "rbd component namespace")
	fs.StringVar(&a.RbdRepoName, "rbd-repo", "rbd-repo", "rbd component repo's name")

}

//SetLog 设置log
func (a *Builder) SetLog() {
	level, err := logrus.ParseLevel(a.LogLevel)
	if err != nil {
		fmt.Println("set log level error." + err.Error())
		return
	}
	logrus.SetLevel(level)
}

//CheckConfig check config
func (a *Builder) CheckConfig() error {
	if a.Topic != client.BuilderTopic && a.Topic != client.WindowsBuilderTopic {
		return fmt.Errorf("Topic is only suppory `%s` and `%s`", client.BuilderTopic, client.WindowsBuilderTopic)
	}
	return nil
}
