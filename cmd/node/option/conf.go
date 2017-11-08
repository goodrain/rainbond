// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

	"github.com/goodrain/rainbond/pkg/node/utils"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/fsnotify/fsnotify"

	"github.com/goodrain/rainbond/pkg/node/event"

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
	if err := Config.watch(); err != nil {
		return err
	}
	initialized = true
	return nil
}

//Conf Conf
type Conf struct {
	APIAddr     string //api server listen port
	K8SConfPath string //absolute path to the kubeconfig file
	LogLevel    string
	RunMode     string //master,node
	Service     string //服务注册与发现
	InitStatus  string
	Node        string // compute node 注册地址
	Master      string // master node 注册地址
	Proc        string // 当前执行任务路径//不知道干吗的
	//任务执行公共路径，后续跟节点ID
	TaskPath            string
	Cmd                 string // 节点执行任务保存路径
	Once                string // 马上执行任务路径//立即执行任务保存地址
	Lock                string // job lock 路径
	Group               string // 节点分组
	Noticer             string // 通知
	EventLogServer      []string
	ExecutionRecordPath string
	ConfigPath          string
	ConfigStorage       string
	K8SNode             string
	BuildIn             string
	BuildInExec         string
	CompJobStatus       string
	FailTime            int
	CheckIntervalSec    int
	InstalledMarker     string

	TTL        int64 // 节点超时时间，单位秒
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

	Etcd client.Config
	//Mail *MailConf
	//
	//Security *Security
}

//AddFlags AddFlags
func (a *Conf) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&a.LogLevel, "log-level", "info", "the log level")
	fs.StringVar(&a.Node, "nodePath", "/acp_node/node/", "the path of node in etcd")
	fs.StringVar(&a.Master, "masterPath", "/acp_node/master/", "the path of master node in etcd")
	fs.StringVar(&a.Proc, "procPath", "/acp_node/proc/", "the path of proc in etcd")
	fs.StringVar(&a.ExecutionRecordPath, "execRecordPath", "/acp_node/exec_record/", "the path of job exec record")
	fs.StringSliceVar(&a.EventLogServer, "event-log-server", []string{"127.0.0.1:6367"}, "host:port slice of event log server")
	fs.StringVar(&a.K8SNode, "k8sNode", "/store/nodes/", "the path of k8s node")
	fs.StringVar(&a.InstalledMarker, "installed-marker", "/etc/acp_node/check/install/success", "the path of a file for check node is installed")
	fs.StringVar(&a.BuildIn, "build-in-jobs", "/store/buildin/", "the path of build-in job")
	fs.StringVar(&a.CompJobStatus, "jobStatus", "/store/jobStatus/", "the path of tree node install status")
	fs.StringVar(&a.BuildInExec, "build-in-exec", "/acp_node/exec_buildin/", "the path of build-in job to watch")
	fs.StringVar(&a.ConfigPath, "configPath", "/acp_node/config/", "the path of config to store")
	fs.StringVar(&a.ConfigStorage, "ConfigStorage", "/acp_node/acp_configs/", "the path of config to store(new)")
	fs.StringVar(&a.InitStatus, "init-status", "/acp_node/init_status/", "the path of init status to store")
	fs.StringVar(&a.Service, "servicePath", "/traefik/backends", "the path of service info to store")
	fs.StringVar(&a.Cmd, "cmdPath", "/acp_node/cmd/", "the path of cmd in etcd")
	fs.StringVar(&a.Once, "oncePath", "/acp_node/once/", "the path of once in etcd")
	fs.StringVar(&a.Lock, "lockPath", "/acp_node/lock/", "the path of lock in etcd")
	fs.StringVar(&a.Group, "groupPath", "/acp_node/group/", "the path of group in etcd")
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
	fs.StringVar(&a.K8SConfPath, "kube-conf", "", "absolute path to the kubeconfig file  ./kubeconfig")
	//fs.StringVar(&a.PrometheusMetricPath, "metric", "/metrics", "prometheus metrics path")
	fs.StringVar(&a.RunMode, "run-mode", "worker", "the entrance run mode,could be 'worker' or 'master'")
	//fs.StringSliceVar(&a.EventServerAddress, "event-servers", []string{"http://127.0.0.1:6363"}, "event message server address.")
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

func (c *Conf) parse() error {
	err := utils.LoadExtendConf(*confFile, c)
	if err != nil {
		return err
	}

	if c.Etcd.DialTimeout > 0 {
		c.Etcd.DialTimeout *= time.Second
	}
	if c.TTL <= 0 {
		c.TTL = 10
	}
	if c.LockTTL < 2 {
		c.LockTTL = 300
	}
	//if c.Mail.Keepalive <= 0 {
	//	c.Mail.Keepalive = 30
	//}
	//if c.Mgo.Timeout <= 0 {
	//	c.Mgo.Timeout = 10 * time.Second
	//} else {
	//	c.Mgo.Timeout *= time.Second
	//}

	c.Node = cleanKeyPrefix(c.Node)
	c.Proc = cleanKeyPrefix(c.Proc)
	c.Cmd = cleanKeyPrefix(c.Cmd)
	c.Once = cleanKeyPrefix(c.Once)
	c.Lock = cleanKeyPrefix(c.Lock)
	c.Group = cleanKeyPrefix(c.Group)
	c.Noticer = cleanKeyPrefix(c.Noticer)

	return nil
}

func (c *Conf) watch() error {
	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		duration := 3 * time.Second
		timer, update := time.NewTimer(duration), false
		for {
			select {
			case <-exitChan:
				return
			case event := <-watcher.Events:
				// 保存文件时会产生多个事件
				if event.Op&(fsnotify.Write|fsnotify.Chmod) > 0 {
					update = true
				}
				timer.Reset(duration)
			case <-timer.C:
				if update {
					c.reload()
					event.Emit(event.WAIT, nil)
					update = false
				}
				timer.Reset(duration)
			case err := <-watcher.Errors:
				logrus.Warnf("config watcher err: %v", err)
			}
		}
	}()

	return watcher.Add(*confFile)
}

// 重新加载配置项
// 注：与系统资源相关的选项不生效，需重启程序
// Etcd
// Mgo
// Web
func (c *Conf) reload() {
	cf := new(Conf)
	if err := cf.parse(); err != nil {
		logrus.Warnf("config file reload err: %s", err.Error())
		return
	}

	// etcd key 选项需要重启
	cf.Node, cf.Proc, cf.Cmd, cf.Once, cf.Lock, cf.Group, cf.Noticer = c.Node, c.Proc, c.Cmd, c.Once, c.Lock, c.Group, c.Noticer

	*c = *cf
	logrus.Infof("config file[%s] reload success", *confFile)
	return
}

func Exit(i interface{}) {
	close(exitChan)
	if watcher != nil {
		watcher.Close()
	}
}
