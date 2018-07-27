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
	"flag"
	"fmt"
	"path"
	"time"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/fsnotify/fsnotify"

	"github.com/spf13/pflag"
)

var (
	confFile = flag.String("conf",
		"conf/files/base.json", "config file path")

	Config      = new(Conf)
	initialized bool

	watcher  *fsnotify.Watcher
	exitChan = make(chan struct{})
)

//Init  初始化
func Init() error {
	if initialized {
		return nil
	}
	Config.AddFlags(pflag.CommandLine)
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
	APIAddr             string //api server listen port
	PrometheusAPI       string //Prometheus server listen port
	K8SConfPath         string //absolute path to the kubeconfig file
	LogLevel            string
	HostIDFile          string
	HostIP              string
	RunMode             string //ACP_NODE 运行模式:master,node
	NodeRule            string //节点属性 compute manage storage
	Service             string //服务注册与发现
	InitStatus          string
	NodePath            string //永久节点信息存储路径
	OnlineNodePath      string //上线节点信息存储路径
	Proc                string // 当前节点正在执行任务存储路径
	StaticTaskPath      string // 配置静态task文件宿主机路径
	JobPath             string // 节点执行任务保存路径
	Lock                string // job lock 路径
	Group               string // 节点分组
	Noticer             string // 通知
	EventLogServer      []string
	ExecutionRecordPath string
	ConfigStoragePath   string
	K8SNode             string
	BuildIn             string
	BuildInExec         string
	CompJobStatus       string
	FailTime            int
	CheckIntervalSec    int
	InstalledMarker     string
	DBType              string
	DBConnectionInfo    string

	TTL        int64 // node heartbeat to master TTL
	ReqTimeout int   // 请求超时时间，单位秒
	// 执行任务信息过期时间，单位秒
	// 0 为不过期
	ProcTTL int64
	// 记录任务执行中的信息的执行时间阀值，单位秒
	// 0 为不限制
	ProcReq int64
	// 单机任务锁过期时间，单位秒
	// 默认 300
	LockTTL int64

	Etcd             client.Config
	StatsdConfig     StatsdConfig
	UDPMonitorConfig UDPMonitorConfig
	MinResyncPeriod  time.Duration

	// for node controller
	ServiceListFile        string
	ServiceEndpointRegPath string
	ServiceManager         string
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
	fs.StringVar(&a.PrometheusAPI, "prometheus", "http://localhost:9999", "the prometheus api")
	fs.StringVar(&a.NodePath, "nodePath", "/rainbond/nodes", "the path of node in etcd")
	fs.StringVar(&a.HostIDFile, "nodeid-file", "/opt/rainbond/etc/node/node_host_uuid.conf", "the unique ID for this node. Just specify, don't modify")
	fs.StringVar(&a.OnlineNodePath, "onlineNodePath", "/rainbond/onlinenodes", "the path of master node in etcd")
	fs.StringVar(&a.Proc, "procPath", "/rainbond/task/proc/", "the path of proc in etcd")
	fs.StringVar(&a.HostIP, "hostIP", "", "the host ip you can define. default get ip from eth0")
	fs.StringVar(&a.ExecutionRecordPath, "execRecordPath", "/rainbond/exec_record", "the path of job exec record")
	fs.StringSliceVar(&a.EventLogServer, "event-log-server", []string{"127.0.0.1:6367"}, "host:port slice of event log server")
	fs.StringVar(&a.K8SNode, "k8sNode", "/store/nodes/", "the path of k8s node")
	fs.StringVar(&a.InstalledMarker, "installed-marker", "/etc/acp_node/check/install/success", "the path of a file for check node is installed")
	fs.StringVar(&a.ConfigStoragePath, "config-path", "/rainbond/acp_configs", "the path of config to store(new)")
	fs.StringVar(&a.InitStatus, "init-status", "/rainbond/init_status", "the path of init status to store")
	fs.StringVar(&a.Service, "servicePath", "/traefik/backends", "the path of service info to store")
	fs.StringVar(&a.JobPath, "jobPath", "/rainbond/jobs", "the path of job in etcd")
	fs.StringVar(&a.Lock, "lockPath", "/rainbond/lock", "the path of lock in etcd")
	fs.IntVar(&a.FailTime, "failTime", 3, "the fail time of healthy check")
	fs.IntVar(&a.CheckIntervalSec, "checkInterval-second", 5, "the interval time of healthy check")
	fs.StringSliceVar(&a.Etcd.Endpoints, "etcd", []string{"http://127.0.0.1:2379"}, "the path of node in etcd")
	fs.DurationVar(&a.Etcd.DialTimeout, "etcd-dialTimeOut", 2*time.Second, "etcd cluster dialTimeOut.")
	fs.IntVar(&a.ReqTimeout, "reqTimeOut", 2, "req TimeOut.")
	fs.Int64Var(&a.TTL, "ttl", 10, "node timeout second")
	fs.Int64Var(&a.ProcTTL, "procttl", 600, "proc ttl")
	fs.Int64Var(&a.ProcReq, "procreq", 5, "proc req")
	fs.Int64Var(&a.LockTTL, "lockttl", 600, "lock ttl")
	fs.StringVar(&a.APIAddr, "api-addr", ":6100", "the api server listen address")
	fs.StringVar(&a.StaticTaskPath, "static-task-path", "/etc/goodrain/rainbond-node", "the file path of static task")
	fs.StringVar(&a.K8SConfPath, "kube-conf", "", "absolute path to the kubeconfig file  ./kubeconfig")
	//fs.StringVar(&a.PrometheusMetricPath, "metric", "/metrics", "prometheus metrics path")
	fs.StringVar(&a.RunMode, "run-mode", "worker", "the acp_node run mode,could be 'worker' or 'master'")
	fs.StringVar(&a.NodeRule, "noderule", "compute", "current node rule,maybe is `compute` `manage` `storage` ")
	//fs.StringSliceVar(&a.EventServerAddress, "event-servers", []string{"http://127.0.0.1:6363"}, "event message server address.")
	fs.StringVar(&a.DBType, "db-type", "mysql", "db type mysql or etcd")
	fs.StringVar(&a.DBConnectionInfo, "mysql", "admin:admin@tcp(127.0.0.1:3306)/region", "mysql db connection info")
	fs.StringVar(&a.StatsdConfig.StatsdListenAddress, "statsd.listen-address", "", "The UDP address on which to receive statsd metric lines. DEPRECATED, use statsd.listen-udp instead.")
	fs.StringVar(&a.StatsdConfig.StatsdListenUDP, "statsd.listen-udp", ":9125", "The UDP address on which to receive statsd metric lines. \"\" disables it.")
	fs.StringVar(&a.StatsdConfig.StatsdListenTCP, "statsd.listen-tcp", ":9125", "The TCP address on which to receive statsd metric lines. \"\" disables it.")
	fs.StringVar(&a.StatsdConfig.MappingConfig, "statsd.mapping-config", "", "Metric mapping configuration file name.")
	fs.IntVar(&a.StatsdConfig.ReadBuffer, "statsd.read-buffer", 0, "Size (in bytes) of the operating system's transmit read buffer associated with the UDP connection. Please make sure the kernel parameters net.core.rmem_max is set to a value greater than the value specified.")
	fs.DurationVar(&a.MinResyncPeriod, "min-resync-period", time.Hour*12, "The resync period in reflectors will be random between MinResyncPeriod and 2*MinResyncPeriod")
	fs.StringVar(&a.ServiceListFile, "service-list-file", "/opt/rainbond/conf/manager-services.yaml", "A list of the node include components")
	fs.StringVar(&a.ServiceEndpointRegPath, "service-endpoint-reg-path", "/rainbond/nodes/target", "For registry service entpoint info into etcd then path.")
	fs.StringVar(&a.ServiceManager, "service-manager", "systemd", "For service management tool on the system.")
}

//SetLog 设置log
func (a *Conf) SetLog() {
	level, err := logrus.ParseLevel(a.LogLevel)
	if err != nil {
		fmt.Println("set log level error." + err.Error())
		return
	}
	logrus.SetLevel(level)
}

type webConfig struct {
	BindAddr string
	UIDir    string
	Auth     struct {
		Enabled bool
	}
	Session SessionConfig
}

type SessionConfig struct {
	Expiration      int
	CookieName      string
	StorePrefixPath string
}

// 返回前后包含斜杆的 /a/b/ 的前缀
func cleanKeyPrefix(p string) string {
	p = path.Clean(p)
	if p[0] != '/' {
		p = "/" + p
	}

	p += "/"

	return p
}

//parse parse
func (c *Conf) parse() error {
	if c.Etcd.DialTimeout > 0 {
		c.Etcd.DialTimeout *= time.Second
	}
	if c.TTL <= 0 {
		c.TTL = 10
	}
	if c.LockTTL < 2 {
		c.LockTTL = 300
	}
	return nil
}
