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
	"github.com/spf13/pflag"
	"github.com/Sirupsen/logrus"
	"fmt"
	"os"
)

type Config struct {
	EtcdEndpoints []string
	LogLevel      string
	ConfigFile    string
	BindIp        string
	Port          int
}

func NewConfig() *Config {
	h, _ := os.Hostname()
	return &Config{
		EtcdEndpoints: []string{"http://127.0.0.1:2379"},
		LogLevel:      "info",
		ConfigFile:    "/etc/prometheus/prometheus.yml",
		BindIp:        h,
		Port:          9999,
	}
}

func (c *Config) AddFlag(cmd *pflag.FlagSet) {
	cmd.StringArrayVar(&c.EtcdEndpoints, "etcd-endpoints", c.EtcdEndpoints, "etcd endpoints list")
	cmd.StringVar(&c.LogLevel, "log-level", c.LogLevel, "log level")
	cmd.StringVar(&c.ConfigFile, "config-file", c.ConfigFile, "prometheus config file path")
	cmd.StringVar(&c.BindIp, "bind-ip", c.BindIp, "prometheus bind ip")
	cmd.IntVar(&c.Port, "port", c.Port, "prometheus listen port")
}

func (c *Config) CompleteConfig() {
	level, err := logrus.ParseLevel(c.LogLevel)
	if err != nil {
		fmt.Println("ERROR set log level:", err)
		return
	}
	logrus.SetLevel(level)

}
