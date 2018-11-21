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
import "github.com/Sirupsen/logrus"
import "fmt"

//Config config server
type Config struct {
	DefaultPluginName    string
	DefaultPluginOpts    []string
	EtcdEndPoints        []string
	EtcdTimeout          int
	EtcdPrefix           string
	ClusterName          string
	APIAddr              string
	RegionAPIAddr        string
	Token                string
	K8SConfPath          string
	NginxHTTPAPI         []string
	NginxStreamAPI       []string
	PrometheusMetricPath string
	EventServerAddress   []string
	BindIP               string
	BindPort             int
	RegTime              int64
	HostIP               string
	HostName             string
	Debug                bool
}

//ACPLBServer lb worker server
type ACPLBServer struct {
	Config
	LogLevel string
	RunMode  string //default,sync
}

//NewACPLBServer new server
func NewACPLBServer() *ACPLBServer {
	return &ACPLBServer{}
}

//AddFlags config
func (a *ACPLBServer) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&a.LogLevel, "log-level", "info", "the entrance log level")
	fs.StringVar(&a.DefaultPluginName, "plugin-name", "zeus", "default lb plugin to be used.")
	fs.StringSliceVar(&a.DefaultPluginOpts, "plugin-opts", []string{}, "default lb plugin options.")
	fs.StringSliceVar(&a.EtcdEndPoints, "etcd-endpoints", []string{"http://127.0.0.1:2379"}, "etcd cluster endpoints.")
	fs.IntVar(&a.EtcdTimeout, "etcd-timeout", 5, "etcd http timeout seconds")
	fs.StringVar(&a.EtcdPrefix, "etcd-prefix", "/entrance", "the etcd data save key prefix ")
	fs.StringVar(&a.ClusterName, "cluster-name", "", "the instance name in cluster ")
	fs.StringVar(&a.APIAddr, "api-addr", ":6200", "the api server listen address")
	fs.StringVar(&a.Token, "token", "", "zeus api token")
	// fs.StringVar(&a.RegionAPIAddr, "region-api-addr", "http://region.goodrain.me:8888", "the region api server address.")
	fs.StringVar(&a.K8SConfPath, "kube-conf", "/opt/rainbond/etc/kubernetes/kubecfg/admin.kubeconfig", "absolute path to the kubeconfig file")
	fs.StringSliceVar(&a.NginxHTTPAPI, "nginx-http", []string{}, "Nginx lb http api.")
	fs.StringSliceVar(&a.NginxStreamAPI, "nginx-stream", []string{}, "Nginx stream api.")
	fs.StringVar(&a.PrometheusMetricPath, "metric", "/metrics", "prometheus metrics path")
	fs.StringVar(&a.RunMode, "run-mode", "default", "the entrance run mode,could be 'default' or 'sync'")
	fs.StringSliceVar(&a.EventServerAddress, "event-servers", []string{"http://127.0.0.1:6363"}, "event message server address.")
	fs.StringVar(&a.BindIP, "bind-ip", "127.0.0.1", "register ip to etcd, with bind-port")
	fs.IntVar(&a.BindPort, "bind-port", 6200, "register port to etcd, need bind-ip")
	fs.Int64Var(&a.RegTime, "ttl", 30, "register keepalive time")
	fs.StringVar(&a.HostIP, "hostIP", "", "Current node Intranet IP")
	fs.StringVar(&a.HostName, "hostName", "", "Current node host name")
	fs.BoolVar(&a.Debug, "debug", false, "if debug true will open pprof")
}

//SetLog 设置log
func (a *ACPLBServer) SetLog() {
	level, err := logrus.ParseLevel(a.LogLevel)
	if err != nil {
		fmt.Println("set log level error." + err.Error())
		return
	}
	logrus.SetLevel(level)
}
