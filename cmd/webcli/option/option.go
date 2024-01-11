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

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

// Config config server
type Config struct {
	EtcdEndPoints        []string
	EtcdCaFile           string
	EtcdCertFile         string
	EtcdKeyFile          string
	Address              string
	HostIP               string
	HostName             string
	Port                 int
	SessionKey           string
	PrometheusMetricPath string
	K8SConfPath          string
}

// WebCliServer container webcli server
type WebCliServer struct {
	Config
	LogLevel string
}

// NewWebCliServer new server
func NewWebCliServer() *WebCliServer {
	return &WebCliServer{}
}

// AddFlags config
func (a *WebCliServer) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&a.LogLevel, "log-level", "info", "the webcli log level")
	fs.StringSliceVar(&a.EtcdEndPoints, "etcd-endpoints", []string{"http://rbd-etcd:2379"}, "etcd v3 cluster endpoints.")
	fs.StringVar(&a.EtcdCaFile, "etcd-ca", "", "etcd tls ca file ")
	fs.StringVar(&a.EtcdCertFile, "etcd-cert", "", "etcd tls cert file")
	fs.StringVar(&a.EtcdKeyFile, "etcd-key", "", "etcd http tls cert key file")
	fs.StringVar(&a.Address, "address", "0.0.0.0", "server listen address")
	fs.StringVar(&a.HostIP, "hostIP", "", "Current node Intranet IP")
	fs.StringVar(&a.HostName, "hostName", "", "Current node host name")
	fs.StringVar(&a.K8SConfPath, "kube-conf", "", "absolute path to the kubeconfig file")
	fs.IntVar(&a.Port, "port", 7171, "server listen port")
	fs.StringVar(&a.PrometheusMetricPath, "metric", "/metrics", "prometheus metrics path")
}

// SetLog 设置log
func (a *WebCliServer) SetLog() {
	level, err := logrus.ParseLevel(a.LogLevel)
	if err != nil {
		fmt.Println("set log level error." + err.Error())
		return
	}
	logrus.SetLevel(level)
}
