// Copyright (C) 2014-2021 Goodrain Co., Ltd.
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
	"fmt"
	"os"
	"path"

	dockercli "github.com/docker/docker/client"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var (

	//Config config
	Config      = new(Conf)
	initialized bool
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
	APIAddr          string //api server listen port
	GrpcAPIAddr      string //grpc api server listen port
	LogLevel         string
	LogFile          string
	StatsdConfig     StatsdConfig
	UDPMonitorConfig UDPMonitorConfig
	//enable collect docker container log
	EnableCollectLog bool
	DockerCli        *dockercli.Client
	// Namespace for Rainbond application.
	RbdNamespace        string
	ImageRepositoryHost string
	GatewayVIP          string
	HostsFile           string
	KubeConfigPath      string
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
	fs.StringVar(&a.GrpcAPIAddr, "grpc-api-addr", ":6101", "The node grpc api server listen address")
	fs.StringVar(&a.StatsdConfig.StatsdListenAddress, "statsd.listen-address", "", "The UDP address on which to receive statsd metric lines. DEPRECATED, use statsd.listen-udp instead.")
	fs.StringVar(&a.StatsdConfig.StatsdListenUDP, "statsd.listen-udp", ":9125", "The UDP address on which to receive statsd metric lines. \"\" disables it.")
	fs.StringVar(&a.StatsdConfig.StatsdListenTCP, "statsd.listen-tcp", ":9125", "The TCP address on which to receive statsd metric lines. \"\" disables it.")
	fs.StringVar(&a.StatsdConfig.MappingConfig, "statsd.mapping-config", "", "Metric mapping configuration file name.")
	fs.IntVar(&a.StatsdConfig.ReadBuffer, "statsd.read-buffer", 0, "Size (in bytes) of the operating system's transmit read buffer associated with the UDP connection. Please make sure the kernel parameters net.core.rmem_max is set to a value greater than the value specified.")
	fs.BoolVar(&a.EnableCollectLog, "enabel-collect-log", true, "Whether to collect container logs")
	fs.StringVar(&a.RbdNamespace, "rbd-ns", "rbd-system", "The namespace of rainbond applications.")
	fs.StringVar(&a.ImageRepositoryHost, "image-repo-host", "goodrain.me", "The host of image repository")
	fs.StringVar(&a.GatewayVIP, "gateway-vip", "", "The vip of gateway")
	fs.StringVar(&a.HostsFile, "hostsfile", "/newetc/hosts", "/etc/hosts mapped path in the container. eg. /etc/hosts:/tmp/hosts. Do not set hostsfile to /etc/hosts")
	fs.StringVar(&a.KubeConfigPath, "kubeconfig", "", "path to kubeconfig file with authorization and master location information.")
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

//ParseClient handle config and create some api
func (a *Conf) ParseClient(ctx context.Context) (err error) {
	a.DockerCli, err = dockercli.NewClientWithOpts(dockercli.FromEnv)
	if err != nil {
		return err
	}
	return nil
}

//parse parse
func (a *Conf) parse() error {
	//init api listen port, can not custom
	if a.APIAddr == "" {
		a.APIAddr = ":6100"
	}
	return nil
}
