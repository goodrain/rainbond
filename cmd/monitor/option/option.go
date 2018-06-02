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
	"strings"
	"strconv"
)

type Config struct {
	EtcdEndpoints []string
	LogLevel      string
	ConfigFile    string
	BindIp        string
	Port          int
	Options       string
	Args          []string
}

func NewConfig() *Config {
	host, _ := os.Hostname()

	config := &Config{
		EtcdEndpoints: []string{"http://127.0.0.1:2379"},
		ConfigFile:    "/etc/prometheus/prometheus.yml",
		BindIp:        host,
		Port:          9999,
		LogLevel:      "info",
	}

	defaultOptions := "--web.listen-address=%s:%d --config.file=%s --storage.tsdb.path=/prometheusdata --storage.tsdb.retention=7d --log.level=%s"
	defaultOptions = fmt.Sprintf(defaultOptions, config.BindIp, config.Port, config.ConfigFile, config.LogLevel)

	config.Options = defaultOptions
	return config
}

func (c *Config) AddFlag(cmd *pflag.FlagSet) {
	cmd.StringArrayVar(&c.EtcdEndpoints, "etcd-endpoints", c.EtcdEndpoints, "etcd endpoints list")
	cmd.StringVar(&c.Options, "prometheus-options", c.Options, "specified options for prometheus")
}

func (c *Config) CompleteConfig() {
	// parse values from prometheus options to config
	args := strings.Split(c.Options, " ")
	for i := 0; i < len(args); i++  {
		kv := strings.Split(args[i], "=")
		if len(kv) < 2 {
			kv = append(kv, args[i])
			i++
		}

		switch kv[0] {
		case "--web.listen-address":
			ipPort := strings.Split(kv[1], ":")
			if ipPort[0] != "" {
				c.BindIp = ipPort[0]
			}
			port, err := strconv.Atoi(ipPort[1])
			if err == nil && port != 0 {
				c.Port = port
			}
		case "--config.file":
			c.ConfigFile = kv[1]
		case "--log.level":
			c.LogLevel = kv[1]
		}
	}

	c.Args = append(c.Args, os.Args[0])
	c.Args = append(c.Args, args...)

	level, err := logrus.ParseLevel(c.LogLevel)
	if err != nil {
		fmt.Println("ERROR set log level:", err)
		return
	}
	logrus.SetLevel(level)

	logrus.Info("Start with options: ", c)
}
