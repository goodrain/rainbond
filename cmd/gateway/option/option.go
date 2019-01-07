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
	"time"

	"github.com/goodrain/rainbond/util"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/pflag"
)

// GWServer contains Config and LogLevel
type GWServer struct {
	Config
	LogLevel string
}

// NewGWServer creates a new option.GWServer
func NewGWServer() *GWServer {
	return &GWServer{}
}

//Config contains all configuration
type Config struct {
	K8SConfPath  string
	EtcdEndpoint []string
	EtcdTimeout  int
	ListenPorts  ListenPorts
	//This number should be, at maximum, the number of CPU cores on your system.
	WorkerProcesses    int
	WorkerRlimitNofile int
	ErrorLog           string
	WorkerConnections  int
	//essential for linux, optmized to serve many clients with each thread
	EnableEpool       bool
	EnableMultiAccept bool
	KeepaliveTimeout  int
	KeepaliveRequests int
	NginxUser         string
	IP                string
	ResyncPeriod      time.Duration
	// health check
	HealthPath         string
	HealthCheckTimeout time.Duration

	EnableMetrics bool

	EnableRbdEndpoints bool
	RbdEndpointsKey    string // key of Rainbond endpoints in ETCD
	EnableKApiServer   bool
	KApiServerIP       string
	EnableLangGrMe     bool
	LangGrMeIP         string
	EnableMVNGrMe      bool
	MVNGrMeIP          string
	EnableGrMe         bool
	GrMeIP             string
	EnableRepoGrMe     bool
	RepoGrMeIP         string
}

// ListenPorts describe the ports required to run the gateway controller
type ListenPorts struct {
	HTTP   int
	HTTPS  int
	Status int
	Health int
}

// AddFlags adds flags
func (g *GWServer) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&g.LogLevel, "log-level", "debug", "the gateway log level")
	fs.StringVar(&g.K8SConfPath, "kube-conf", "/opt/rainbond/etc/kubernetes/kubecfg/admin.kubeconfig", "absolute path to the kubeconfig file")
	fs.IntVar(&g.ListenPorts.Status, "status-port", 18080, `Port to use for exposing NGINX status pages.`)
	fs.IntVar(&g.WorkerProcesses, "worker-processes", 0, "Default get current compute cpu core number.This number should be, at maximum, the number of CPU cores on your system.")
	fs.IntVar(&g.WorkerConnections, "worker-connections", 4000, "Determines how many clients will be served by each worker process.")
	fs.IntVar(&g.WorkerRlimitNofile, "worker-rlimit-nofile", 200000, "Number of file descriptors used for Nginx. This is set in the OS with 'ulimit -n 200000'")
	fs.BoolVar(&g.EnableEpool, "enable-epool", true, "essential for linux, optmized to serve many clients with each thread")
	fs.BoolVar(&g.EnableMultiAccept, "enable-multi-accept", true, "Accept as many connections as possible, after nginx gets notification about a new connection.")
	fs.StringVar(&g.ErrorLog, "error-log", "/dev/stderr crit", "only log critical errors")
	fs.StringVar(&g.NginxUser, "nginx-user", "root", "nginx user name")
	fs.IntVar(&g.KeepaliveRequests, "keepalive-requests", 10000, "Number of requests a client can make over the keep-alive connection. ")
	fs.IntVar(&g.KeepaliveTimeout, "keepalive-timeout", 30, "Timeout for keep-alive connections. Server will close connections after this time.")
	fs.StringVar(&g.IP, "ip", "0.0.0.0", "Node ip.") // TODO: more detail
	fs.DurationVar(&g.ResyncPeriod, "resync-period", 10*time.Second, "the default resync period for any handlers added via AddEventHandler and how frequently the listener wants a full resync from the shared informer")
	// etcd
	fs.StringSliceVar(&g.EtcdEndpoint, "etcd-endpoints", []string{"http://127.0.0.1:2379"}, "etcd cluster endpoints.")
	fs.IntVar(&g.EtcdTimeout, "etcd-timeout", 5, "etcd http timeout seconds")
	// health check
	fs.StringVar(&g.HealthPath, "health-path", "/healthz", "absolute path to the kubeconfig file")
	fs.DurationVar(&g.HealthCheckTimeout, "health-check-timeout", 10, `Time limit, in seconds, for a probe to health-check-path to succeed.`)
	fs.IntVar(&g.ListenPorts.Health, "healthz-port", 10254, `Port to use for the healthz endpoint.`)
	fs.BoolVar(&g.EnableMetrics, "enable-metrics", true, "Enables the collection of rbd-gateway metrics")
	// rainbond endpoints
	fs.BoolVar(&g.EnableRbdEndpoints, "enable-rbd-endpoints", true, "switch of Rainbond endpoints")
	fs.StringVar(&g.RbdEndpointsKey, "rbd-endpoints", "/rainbond/endpoint/", "key of Rainbond endpoints in ETCD")
	fs.BoolVar(&g.EnableKApiServer, "enable-kubeapi", false, "enable load balancing of kube-apiserver")
	fs.StringVar(&g.KApiServerIP, "kubeapi-ip", "0.0.0.0", "ip address bound by kube-apiserver")
	fs.BoolVar(&g.EnableLangGrMe, "enable-lang-grme", true, "enable load balancing of lang.goodrain.me")
	fs.StringVar(&g.LangGrMeIP, "lang-grme-ip", "0.0.0.0", "ip address bound by lang.goodrain.me")
	fs.BoolVar(&g.EnableMVNGrMe, "enable-mvn-grme", true, "enable load balancing of maven.goodrain.me")
	fs.StringVar(&g.MVNGrMeIP, "mvn-grme-ip", "0.0.0.0", "ip address bound by maven.goodrain.me")
	fs.BoolVar(&g.EnableGrMe, "enable-grme", true, "enable load balancing of goodrain.me")
	fs.StringVar(&g.GrMeIP, "grme-ip", "0.0.0.0", "ip address bound by goodrain.me")
	fs.BoolVar(&g.EnableRepoGrMe, "enable-repo-grme", true, "enable load balancing of repo.goodrain.me")
	fs.StringVar(&g.RepoGrMeIP, "repo-grme-ip", "0.0.0.0", "ip address bound by repo.goodrain.me")
}

// SetLog sets log
func (g *GWServer) SetLog() {
	level, err := logrus.ParseLevel(g.LogLevel)
	if err != nil {
		fmt.Println("set log level error." + err.Error())
		return
	}
	logrus.SetLevel(level)
}

//CheckConfig check config
func (g *GWServer) CheckConfig() error {
	if g.K8SConfPath == "" {
		return fmt.Errorf("kube config file path can not be empty")
	}
	if exist, _ := util.FileExists(g.K8SConfPath); !exist {
		return fmt.Errorf("kube config file %s not exist", g.K8SConfPath)
	}
	return nil
}
