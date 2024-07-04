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

// Config config
type Config struct {
	DBType                 string
	DBInterpolateParams    string
	APIAddr                string
	APIHealthzAddr         string
	APIAddrSSL             string
	DBConnectionInfo       string
	EventLogServers        []string
	DockerConsoleServers   []string
	EventLogEndpoints      []string
	NodeAPI                []string
	BuilderAPI             []string
	V1API                  string
	MQAPI                  string
	EtcdEndpoint           []string
	EtcdCaFile             string
	EtcdCertFile           string
	EtcdKeyFile            string
	APISSL                 bool
	APICertFile            string
	APIKeyFile             string
	APICaFile              string
	WebsocketSSL           bool
	WebsocketCertFile      string
	WebsocketKeyFile       string
	WebsocketAddr          string
	Opentsdb               string
	RegionTag              string
	LoggerFile             string
	EnableFeature          []string
	Debug                  bool
	MinExtPort             int // minimum external port
	LicensePath            string
	LicSoPath              string
	LogPath                string
	KuberentesDashboardAPI string
	KubeConfigPath         string
	PrometheusEndpoint     string
	RbdNamespace           string
	ShowSQL                bool
	GrctlImage             string
	RbdHub                 string
	RbdWorker              string
}

// APIServer  apiserver server
type APIServer struct {
	Config
	LogLevel       string
	StartRegionAPI bool
}

// NewAPIServer new server
func NewAPIServer() *APIServer {
	return &APIServer{}
}

// AddFlags config
func (a *APIServer) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&a.LogLevel, "log-level", "info", "the api log level")
	fs.StringVar(&a.DBType, "db-type", "mysql", "db type mysql or etcd")
	fs.StringVar(&a.DBConnectionInfo, "mysql", "admin:admin@tcp(127.0.0.1:3306)/region", "mysql db connection info")
	fs.StringVar(&a.DBInterpolateParams, "db-interpolate-params", "false", "db interpolate params, for compatible oceanbase, should set true")
	fs.StringVar(&a.APIAddr, "api-addr", "127.0.0.1:8888", "the api server listen address")
	fs.StringVar(&a.APIHealthzAddr, "api-healthz-addr", "0.0.0.0:8889", "the api server health check listen address")
	fs.StringVar(&a.APIAddrSSL, "api-addr-ssl", "0.0.0.0:8443", "the api server listen address")
	fs.StringVar(&a.WebsocketAddr, "ws-addr", "0.0.0.0:6060", "the websocket server listen address")
	fs.BoolVar(&a.APISSL, "api-ssl-enable", false, "whether to enable websocket  SSL")
	fs.StringVar(&a.APICaFile, "client-ca-file", "", "api ssl ca file")
	fs.StringVar(&a.APICertFile, "api-ssl-certfile", "", "api ssl cert file")
	fs.StringVar(&a.APIKeyFile, "api-ssl-keyfile", "", "api ssl cert file")
	fs.BoolVar(&a.WebsocketSSL, "ws-ssl-enable", false, "whether to enable websocket  SSL")
	fs.StringVar(&a.WebsocketCertFile, "ws-ssl-certfile", "/etc/ssl/goodrain.com/goodrain.com.crt", "websocket and fileserver ssl cert file")
	fs.StringVar(&a.WebsocketKeyFile, "ws-ssl-keyfile", "/etc/ssl/goodrain.com/goodrain.com.key", "websocket and fileserver ssl key file")
	fs.StringVar(&a.V1API, "v1-api", "127.0.0.1:8887", "the region v1 api")
	fs.StringSliceVar(&a.BuilderAPI, "builder-api", []string{"rbd-chaos:3228"}, "the builder api")
	fs.BoolVar(&a.StartRegionAPI, "start", false, "Whether to start region old api")
	fs.StringVar(&a.EtcdCaFile, "etcd-ca", "", "verify etcd certificates of TLS-enabled secure servers using this CA bundle")
	fs.StringVar(&a.EtcdCertFile, "etcd-cert", "", "identify secure etcd client using this TLS certificate file")
	fs.StringVar(&a.EtcdKeyFile, "etcd-key", "", "identify secure etcd client using this TLS key file")
	fs.StringVar(&a.Opentsdb, "opentsdb", "127.0.0.1:4242", "opentsdb server config")
	fs.StringVar(&a.RegionTag, "region-tag", "test-ali", "region tag setting")
	fs.StringVar(&a.LoggerFile, "logger-file", "/logs/request.log", "request log file path")
	fs.BoolVar(&a.Debug, "debug", false, "open debug will enable pprof")
	fs.IntVar(&a.MinExtPort, "min-ext-port", 0, "minimum external port")
	fs.StringArrayVar(&a.EnableFeature, "enable-feature", []string{}, "List of special features supported, such as `windows`")
	fs.StringVar(&a.LicensePath, "license-path", "/opt/rainbond/etc/license/license.yb", "the license path of the enterprise version.")
	fs.StringVar(&a.LicSoPath, "license-so-path", "/opt/rainbond/etc/license/license.so", "Dynamic library file path for parsing the license.")
	fs.StringVar(&a.LogPath, "log-path", "/grdata/logs", "Where Docker log files and event log files are stored.")
	fs.StringVar(&a.KubeConfigPath, "kube-config", "", "kube config file path, No setup is required to run in a cluster.")
	fs.StringVar(&a.KuberentesDashboardAPI, "k8s-dashboard-api", "kubernetes-dashboard.rbd-system:443", "The service DNS name of Kubernetes dashboard. Default to kubernetes-dashboard.kubernetes-dashboard")
	fs.StringVar(&a.RbdNamespace, "rbd-namespace", "rbd-system", "rbd component namespace")
	fs.BoolVar(&a.ShowSQL, "show-sql", false, "The trigger for showing sql.")
	fs.StringVar(&a.GrctlImage, "shell-image", "registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-shell:v5.13.0-release", "use shell image")

	fs.StringSliceVar(&a.EtcdEndpoint, "etcd", []string{"http://rbd-etcd:2379"}, "etcd server or proxy address")
	fs.StringSliceVar(&a.DockerConsoleServers, "docker-console", []string{"rbd-webcli:7171"}, "docker console address")
	fs.StringVar(&a.PrometheusEndpoint, "prom-api", "rbd-monitor:9999", "The service DNS name of Prometheus api. Default to rbd-monitor:9999")
	fs.StringVar(&a.RbdHub, "hub-api", "http://rbd-hub:5000", "the rbd-hub server api")
	fs.StringSliceVar(&a.NodeAPI, "node-api", []string{"rbd-node:6100"}, "the rbd-node server api")
	fs.StringVar(&a.MQAPI, "mq-api", "rbd-mq:6300", "the rbd-mq server api")
	fs.StringVar(&a.RbdWorker, "worker-api", "rbd-worker:6535", "the rbd-worker server api")
	fs.StringSliceVar(&a.EventLogServers, "event-servers", []string{"rbd-eventlog:6366"}, "event log server address")
	fs.StringSliceVar(&a.EventLogEndpoints, "event-log", []string{"local=>rbd-eventlog:6363"}, "event log websocket address")

}

// SetLog 设置log
func (a *APIServer) SetLog() {
	level, err := logrus.ParseLevel(a.LogLevel)
	if err != nil {
		fmt.Println("set log level error." + err.Error())
		return
	}
	logrus.Infof("Etcd Server : %+v", a.Config.EtcdEndpoint)
	logrus.SetLevel(level)
}
