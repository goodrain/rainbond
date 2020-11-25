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

import "github.com/spf13/pflag"
import "github.com/sirupsen/logrus"
import "fmt"

//Config config server
type Config struct {
	EtcdEndPoints        []string
	EtcdCaFile           string
	EtcdCertFile         string
	EtcdKeyFile          string
	EtcdTimeout          int
	EtcdPrefix           string
	ClusterName          string
	APIPort              int
	PrometheusMetricPath string
	RunMode              string //http grpc
	HostIP               string
	HostName             string
}

//MQServer lb worker server
type MQServer struct {
	Config
	LogLevel string
}

//NewMQServer new server
func NewMQServer() *MQServer {
	return &MQServer{}
}

//AddFlags config
func (a *MQServer) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&a.LogLevel, "log-level", "info", "the mq log level")
	fs.StringSliceVar(&a.EtcdEndPoints, "etcd-endpoints", []string{"http://127.0.0.1:2379"}, "etcd v3 cluster endpoints.")
	fs.IntVar(&a.EtcdTimeout, "etcd-timeout", 10, "etcd http timeout seconds")
	fs.StringVar(&a.EtcdCaFile, "etcd-ca", "", "etcd tls ca file ")
	fs.StringVar(&a.EtcdCertFile, "etcd-cert", "", "etcd tls cert file")
	fs.StringVar(&a.EtcdKeyFile, "etcd-key", "", "etcd http tls cert key file")
	fs.StringVar(&a.EtcdPrefix, "etcd-prefix", "/mq", "the etcd data save key prefix ")
	fs.IntVar(&a.APIPort, "api-port", 6300, "the api server listen port")
	fs.StringVar(&a.RunMode, "mode", "grpc", "the api server run mode grpc or http")
	fs.StringVar(&a.PrometheusMetricPath, "metric", "/metrics", "prometheus metrics path")
	fs.StringVar(&a.HostIP, "hostIP", "", "Current node Intranet IP")
	fs.StringVar(&a.HostName, "hostName", "", "Current node host name")
}

//SetLog 设置log
func (a *MQServer) SetLog() {
	level, err := logrus.ParseLevel(a.LogLevel)
	if err != nil {
		fmt.Println("set log level error." + err.Error())
		return
	}
	logrus.SetLevel(level)
}
