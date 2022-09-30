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
	"context"
	"flag"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/goodrain/rainbond/builder/sources"
	"os"
	"path"
	"time"

	client "github.com/coreos/etcd/clientv3"
	"github.com/fsnotify/fsnotify"
	"github.com/goodrain/rainbond/util"
	etcdutil "github.com/goodrain/rainbond/util/etcd"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	// Register grpc event types
	_ "github.com/containerd/containerd/api/events"
)

var (
	confFile = flag.String("conf",
		"conf/files/base.json", "config file path")
	//Config config
	Config      = new(Conf)
	initialized bool

	watcher  *fsnotify.Watcher
	exitChan = make(chan struct{})
)

//Init  init config
func Init() error {
	if initialized {
		return nil
	}
	pflag.Parse()
	Config.SetLog()
	if err := Config.parse(); err != nil {
		return err
	}
	initialized = true
	return nil
}

//Conf Conf
type Conf struct {
	APIAddr                         string //api server listen port
	GrpcAPIAddr                     string //grpc api server listen port
	PrometheusAPI                   string //Prometheus server listen port
	K8SConfPath                     string //absolute path to the kubeconfig file
	LogLevel                        string
	LogFile                         string
	HostID                          string
	HostIP                          string
	PodIP                           string
	RunMode                         string //ACP_NODE 运行模式:master,node
	NodeRule                        string //节点属性 compute manage storage
	Service                         string //服务注册与发现
	InitStatus                      string
	NodePath                        string   //Rainbond node model basic information storage path in etcd
	EventLogServer                  []string //event server address list
	ConfigStoragePath               string   //config storage path in etcd
	LockPath                        string
	TTL                             int64         // node heartbeat to master TTL
	PodCIDR                         string        //pod cidr, when master not set cidr,this parameter can take effect
	EtcdEndpoints                   []string      // etcd endpoints
	EtcdDialTimeout                 time.Duration // etcd dial timeout
	EtcdCaFile                      string        // etcd ca file
	EtcdCertFile                    string        // etcd cert file
	EtcdKeyFile                     string        // etcd key file
	StatsdConfig                    StatsdConfig
	UDPMonitorConfig                UDPMonitorConfig
	MinResyncPeriod                 time.Duration
	AutoUnschedulerUnHealthDuration time.Duration
	AutoScheduler                   bool
	ReqTimeout                      int

	// for node controller
	ServiceListFile string
	ServiceManager  string
	EnableInitStart bool
	AutoRegistNode  bool
	//enable collect docker container log
	EnableCollectLog  bool
	ContainerRuntime  string
	RuntimeEndpoint   string
	ContainerImageCli sources.ContainerImageCli
	EtcdCli           *client.Client

	LicPath   string
	LicSoPath string

	// EnableImageGC is the trigger of image garbage collection.
	EnableImageGC bool
	// imageMinimumGCAge is the minimum age for an unused image before it is
	// garbage collected.
	ImageMinimumGCAge time.Duration
	// imageGCHighThresholdPercent is the percent of disk usage after which
	// image garbage collection is always run. The percent is calculated as
	// this field value out of 100.
	ImageGCHighThresholdPercent int32
	// imageGCLowThresholdPercent is the percent of disk usage before which
	// image garbage collection is never run. Lowest disk usage to garbage
	// collect to. The percent is calculated as this field value out of 100.
	ImageGCLowThresholdPercent int32
	// ImageGCPeriod is the period for performing image garbage collection.
	ImageGCPeriod time.Duration

	// Namespace for Rainbond application.
	RbdNamespace        string
	ImageRepositoryHost string
	GatewayVIP          string
	HostsFile           string
}

//StatsdConfig StatsdConfig
type StatsdConfig struct {
	StatsdListenAddress string
	StatsdListenUDP     string
	StatsdListenTCP     string
	MappingConfig       string
	ReadBuffer          int
}

//UDPMonitorConfig UDPMonitorConfig
type UDPMonitorConfig struct {
	ListenHost string
	ListenPort string
}

//AddFlags AddFlags
func (a *Conf) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&a.LogLevel, "log-level", "info", "the log level")
	fs.StringVar(&a.LogFile, "log-file", "", "the log file path that log output")
	fs.StringVar(&a.PrometheusAPI, "prometheus", "http://rbd-monitor:9999", "the prometheus server address")
	fs.StringVar(&a.NodePath, "nodePath", "/rainbond/nodes", "the path of node in etcd")
	fs.StringVar(&a.HostID, "nodeid", "", "the unique ID for this node. Just specify, don't modify")
	fs.StringVar(&a.HostIP, "hostIP", "", "the host ip you can define. default get ip from eth0")
	fs.StringVar(&a.PodIP, "podIP", "", "The pod ip of node.")
	fs.StringSliceVar(&a.EventLogServer, "event-log-server", []string{"127.0.0.1:6366"}, "host:port slice of event log server")
	fs.StringVar(&a.ConfigStoragePath, "config-path", "/rainbond/acp_configs", "the path of config to store(new)")
	fs.StringVar(&a.Service, "servicePath", "/traefik/backends", "the path of service info to store")
	fs.StringSliceVar(&a.EtcdEndpoints, "etcd", []string{"http://127.0.0.1:2379"}, "the path of node in etcd")
	fs.StringVar(&a.EtcdCaFile, "etcd-ca", "", "verify etcd certificates of TLS-enabled secure servers using this CA bundle")
	fs.StringVar(&a.EtcdCertFile, "etcd-cert", "", "identify secure etcd client using this TLS certificate file")
	fs.StringVar(&a.EtcdKeyFile, "etcd-key", "", "identify secure etcd client using this TLS key file")
	fs.DurationVar(&a.EtcdDialTimeout, "etcd-dialTimeOut", 3, "etcd cluster dialTimeOut In seconds")
	fs.IntVar(&a.ReqTimeout, "reqTimeOut", 2, "req TimeOut.")
	fs.Int64Var(&a.TTL, "ttl", 10, "Frequency of node status reporting to master")
	//fs.StringVar(&a.APIAddr, "api-addr", ":6100", "The node api server listen address")
	fs.StringVar(&a.GrpcAPIAddr, "grpc-api-addr", ":6101", "The node grpc api server listen address")
	fs.StringVar(&a.K8SConfPath, "kube-conf", "", "absolute path to the kubeconfig file  ./kubeconfig")
	fs.StringVar(&a.RunMode, "run-mode", "worker", "the acp_node run mode,could be 'worker' or 'master'")
	fs.StringVar(&a.NodeRule, "noderule", "compute", "current node rule,maybe is `compute` `manage` `storage` ")
	fs.StringVar(&a.StatsdConfig.StatsdListenAddress, "statsd.listen-address", "", "The UDP address on which to receive statsd metric lines. DEPRECATED, use statsd.listen-udp instead.")
	fs.StringVar(&a.StatsdConfig.StatsdListenUDP, "statsd.listen-udp", ":9125", "The UDP address on which to receive statsd metric lines. \"\" disables it.")
	fs.StringVar(&a.StatsdConfig.StatsdListenTCP, "statsd.listen-tcp", ":9125", "The TCP address on which to receive statsd metric lines. \"\" disables it.")
	fs.StringVar(&a.StatsdConfig.MappingConfig, "statsd.mapping-config", "", "Metric mapping configuration file name.")
	fs.IntVar(&a.StatsdConfig.ReadBuffer, "statsd.read-buffer", 0, "Size (in bytes) of the operating system's transmit read buffer associated with the UDP connection. Please make sure the kernel parameters net.core.rmem_max is set to a value greater than the value specified.")
	fs.DurationVar(&a.MinResyncPeriod, "min-resync-period", time.Second*15, "The resync period in reflectors will be random between MinResyncPeriod and 2*MinResyncPeriod")
	fs.StringVar(&a.ServiceListFile, "service-list-file", "/opt/rainbond/conf/", "Specifies the configuration file, which can be a directory, that configures the service running on the current node")
	fs.BoolVar(&a.EnableInitStart, "enable-init-start", false, "Enable dependency - free initialization starts for services that support initialization starts")
	fs.BoolVar(&a.AutoRegistNode, "auto-registnode", true, "Whether auto regist node info to cluster where node is not found")
	fs.BoolVar(&a.AutoScheduler, "auto-scheduler", true, "Whether auto set node unscheduler where current node is unhealth")
	fs.BoolVar(&a.EnableCollectLog, "enabel-collect-log", true, "Whether to collect container logs")
	fs.DurationVar(&a.AutoUnschedulerUnHealthDuration, "autounscheduler-unhealthy-dura", 5*time.Minute, "Node unhealthy duration, after the automatic offline,if set 0,disable auto handle unscheduler.default is 5 Minute")
	fs.StringVar(&a.LicPath, "lic-path", "/opt/rainbond/etc/license/license.yb", "the license path of the enterprise version.")
	fs.StringVar(&a.LicSoPath, "lic-so-path", "/opt/rainbond/etc/license/license.so", "Dynamic library file path for parsing the license.")
	fs.BoolVar(&a.EnableImageGC, "enable-image-gc", true, "The trigger of image garbage collection.")
	fs.DurationVar(&a.ImageMinimumGCAge, "minimum-image-ttl-duration", 2*time.Hour, "Minimum age for an unused image before it is garbage collected.  Examples: '300ms', '10s' or '2h45m'.")
	fs.DurationVar(&a.ImageGCPeriod, "image-gc-period", 5*time.Minute, "ImageGCPeriod is the period for performing image garbage collection.  Examples: '10s', '5m' or '2h45m'.")
	fs.Int32Var(&a.ImageGCHighThresholdPercent, "image-gc-high-threshold", 90, "The percent of disk usage after which image garbage collection is always run. Values must be within the range [0, 100], To disable image garbage collection, set to 100. ")
	fs.Int32Var(&a.ImageGCLowThresholdPercent, "image-gc-low-threshold", 75, "The percent of disk usage before which image garbage collection is never run. Lowest disk usage to garbage collect to. Values must be within the range [0, 100] and should not be larger than that of --image-gc-high-threshold.")
	fs.StringVar(&a.RbdNamespace, "rbd-ns", "rbd-system", "The namespace of rainbond applications.")
	fs.StringVar(&a.ImageRepositoryHost, "image-repo-host", "goodrain.me", "The host of image repository")
	fs.StringVar(&a.GatewayVIP, "gateway-vip", "", "The vip of gateway")
	fs.StringVar(&a.HostsFile, "hostsfile", "/newetc/hosts", "/etc/hosts mapped path in the container. eg. /etc/hosts:/tmp/hosts. Do not set hostsfile to /etc/hosts")
	fs.StringVar(&a.ContainerRuntime, "container-runtime", sources.ContainerRuntimeContainerd, "container runtime, support docker and containerd")
	fs.StringVar(&a.RuntimeEndpoint, "runtime-endpoint", sources.RuntimeEndpointContainerd, "container runtime endpoint")
}

//SetLog 设置log
func (a *Conf) SetLog() {
	level, err := logrus.ParseLevel(a.LogLevel)
	if err != nil {
		fmt.Println("set log level error." + err.Error())
		return
	}
	logrus.SetLevel(level)
	if a.LogFile != "" {
		if err := util.CheckAndCreateDir(path.Dir(a.LogFile)); err != nil {
			logrus.Errorf("create node log file dir failure %s", err.Error())
			os.Exit(1)
		}
		logfile, err := os.OpenFile(a.LogFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0755)
		if err != nil {
			logrus.Errorf("create and open node log file failure %s", err.Error())
			os.Exit(1)
		}
		logrus.SetOutput(logfile)
	}
}

func newClient(namespace, address string, opts ...containerd.ClientOpt) (*containerd.Client, context.Context, context.CancelFunc, error) {
	ctx := namespaces.WithNamespace(context.Background(), namespace)
	client, err := containerd.New(address, opts...)
	if err != nil {
		return nil, nil, nil, err
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	return client, ctx, cancel, nil
}

//ParseClient handle config and create some api
func (a *Conf) ParseClient(ctx context.Context, etcdClientArgs *etcdutil.ClientArgs) (err error) {
	logrus.Infof("begin create container image client, runtime [%s] runtime endpoint [%s]", a.ContainerRuntime, a.RuntimeEndpoint, a.EtcdEndpoints)
	containerImageCli, err := sources.NewContainerImageClient(a.ContainerRuntime, a.RuntimeEndpoint, time.Second*3)
	if err != nil {
		return err
	}
	a.ContainerImageCli = containerImageCli
	logrus.Infof("create container image client success\n begin create etcd client: %s", a.EtcdEndpoints)
	for {
		a.EtcdCli, err = etcdutil.NewClient(ctx, etcdClientArgs)
		if err != nil {
			logrus.Errorf("create etcd client failure %s, will retry after 3 second", err.Error())
		}
		if err == nil && a.EtcdCli != nil {
			break
		}
		time.Sleep(time.Second * 3)
	}
	logrus.Infof("create etcd client success")
	return nil
}

//parse parse
func (a *Conf) parse() error {
	if a.TTL <= 0 {
		a.TTL = 10
	}
	a.LockPath = "/rainbond/lock"
	if a.HostIP == "" || !util.CheckIP(a.HostIP) {
		localIP, err := util.LocalIP()
		if localIP == nil || err != nil {
			return fmt.Errorf("can not find ip of this node")
		}
		a.HostIP = localIP.String()
	}
	//init api listen port, can not custom
	if a.APIAddr == "" {
		a.APIAddr = ":6100"
	}
	if a.HostID == "" {
		return fmt.Errorf("kubernetes node id can't empty")
	}
	return nil
}
