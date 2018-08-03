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
	"github.com/Sirupsen/logrus"
	"github.com/spf13/pflag"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	EtcdEndpointsLine string
	EtcdEndpoints     []string
	LogLevel          string
	AdvertiseAddr     string
	BindIp            string
	Port              int

	StartArgs            []string
	ConfigFile           string
	AlertingRulesFile    string
	AlertManagerUrl      []string
	LocalStoragePath     string
	Web                  Web
	Tsdb                 Tsdb
	WebTimeout           string
	RemoteFlushDeadline  string
	AlertmanagerCapacity string
	AlertmanagerTimeout  string
	QueryLookbackDelta   string
	QueryTimeout         string
	QueryMaxConcurrency  string
}

// Options for the web Handler.
type Web struct {
	ListenAddress        string
	ReadTimeout          time.Duration
	MaxConnections       int
	ExternalURL          string
	RoutePrefix          string
	UseLocalAssets       bool
	UserAssetsPath       string
	ConsoleTemplatesPath string
	ConsoleLibrariesPath string
	EnableLifecycle      bool
	EnableAdminAPI       bool
}

// Options of the DB storage.
type Tsdb struct {
	// The interval at which the write ahead log is flushed to disc.
	WALFlushInterval time.Duration

	// The timestamp range of head blocks after which they get persisted.
	// It's the minimum duration of any persisted block.
	MinBlockDuration string

	// The maximum timestamp range of compacted blocks.
	MaxBlockDuration string

	// Duration for how long to retain data.
	Retention string

	// Disable creation and consideration of lockfile.
	NoLockfile bool
}

func NewConfig() *Config {
	host, _ := os.Hostname()

	config := &Config{
		EtcdEndpointsLine: "http://127.0.0.1:2379",
		EtcdEndpoints:     []string{},
		AdvertiseAddr:     host + ":9999",
		BindIp:            host,
		Port:              9999,
		LogLevel:          "info",

		ConfigFile:           "/etc/prometheus/prometheus.yml",
		AlertingRulesFile:    "/etc/prometheus/rules.yml",
		AlertManagerUrl:      []string{},
		LocalStoragePath:     "/prometheusdata",
		WebTimeout:           "5m",
		RemoteFlushDeadline:  "1m",
		AlertmanagerCapacity: "10000",
		AlertmanagerTimeout:  "10s",
		QueryLookbackDelta:   "5m",
		QueryTimeout:         "2m",
		QueryMaxConcurrency:  "20",
		Web: Web{
			ListenAddress:        "0.0.0.0:9999",
			ReadTimeout:          time.Minute * 5,
			MaxConnections:       512,
			ConsoleTemplatesPath: "consoles",
			ConsoleLibrariesPath: "console_libraries",
		},
		Tsdb: Tsdb{
			MinBlockDuration: "2h",
			Retention:        "7d",
		},
	}

	return config
}

func (c *Config) AddFlag(cmd *pflag.FlagSet) {
	cmd.StringVar(&c.EtcdEndpointsLine, "etcd-endpoints", c.EtcdEndpointsLine, "etcd endpoints list.")
	cmd.StringVar(&c.AdvertiseAddr, "advertise-addr", c.AdvertiseAddr, "advertise address, and registry into etcd.")
	cmd.StringSliceVar(&c.AlertManagerUrl, "alertmanager-address", c.AlertManagerUrl, "AlertManager url.")
}

func (c *Config) AddPrometheusFlag(cmd *pflag.FlagSet) {
	cmd.StringVar(&c.ConfigFile, "config.file", c.ConfigFile, "Prometheus configuration file path.")

	cmd.StringVar(&c.AlertingRulesFile, "rules-config.file", c.AlertingRulesFile, "Prometheus alerting rules config file path.")

	cmd.StringVar(&c.Web.ListenAddress, "web.listen-address", c.Web.ListenAddress, "Address to listen on for UI, API, and telemetry.")

	cmd.StringVar(&c.WebTimeout, "web.read-timeout", c.WebTimeout, "Maximum duration before timing out read of the request, and closing idle connections.")

	cmd.IntVar(&c.Web.MaxConnections, "web.max-connections", c.Web.MaxConnections, "Maximum number of simultaneous connections.")

	cmd.StringVar(&c.Web.ExternalURL, "web.external-url", c.Web.ExternalURL, "The URL under which Prometheus is externally reachable (for example, if Prometheus is served via a reverse proxy). Used for generating relative and absolute links back to Prometheus itself. If the URL has a path portion, it will be used to prefix all HTTP endpoints served by Prometheus. If omitted, relevant URL components will be derived automatically.")

	cmd.StringVar(&c.Web.RoutePrefix, "web.route-prefix", c.Web.RoutePrefix, "Prefix for the internal routes of Web endpoints. Defaults to path of --Web.external-url.")

	cmd.StringVar(&c.Web.UserAssetsPath, "web.user-assets", c.Web.UserAssetsPath, "Path to static asset directory, available at /user.")

	cmd.BoolVar(&c.Web.EnableLifecycle, "web.enable-lifecycle", c.Web.EnableLifecycle, "Enable shutdown and reload via HTTP request.")

	cmd.BoolVar(&c.Web.EnableAdminAPI, "web.enable-admin-api", c.Web.EnableAdminAPI, "Enable API endpoints for admin control actions.")

	cmd.StringVar(&c.Web.ConsoleTemplatesPath, "web.console.templates", c.Web.ConsoleTemplatesPath, "Path to the console template directory, available at /consoles.")

	cmd.StringVar(&c.Web.ConsoleLibrariesPath, "web.console.libraries", c.Web.ConsoleLibrariesPath, "Path to the console library directory.")

	cmd.StringVar(&c.LocalStoragePath, "storage.tsdb.path", c.LocalStoragePath, "Base path for metrics storage.")

	cmd.StringVar(&c.Tsdb.MinBlockDuration, "storage.tsdb.min-block-duration", c.Tsdb.MinBlockDuration, "Minimum duration of a data block before being persisted. For use in testing.")

	cmd.StringVar(&c.Tsdb.MaxBlockDuration, "storage.tsdb.max-block-duration", c.Tsdb.MaxBlockDuration,
		"Maximum duration compacted blocks may span. For use in testing. (Defaults to 10% of the retention period).")

	cmd.StringVar(&c.Tsdb.Retention, "storage.tsdb.retention", c.Tsdb.Retention, "How long to retain samples in storage.")

	cmd.BoolVar(&c.Tsdb.NoLockfile, "storage.tsdb.no-lockfile", c.Tsdb.NoLockfile, "Do not create lockfile in data directory.")

	cmd.StringVar(&c.RemoteFlushDeadline, "storage.remote.flush-deadline", c.RemoteFlushDeadline, "How long to wait flushing sample on shutdown or config reload.")

	cmd.StringVar(&c.AlertmanagerCapacity, "alertmanager.notification-queue-capacity", c.AlertmanagerCapacity, "The capacity of the queue for pending Alertmanager notifications.")

	cmd.StringVar(&c.AlertmanagerTimeout, "alertmanager.timeout", c.AlertmanagerTimeout, "Timeout for sending alerts to Alertmanager.")

	cmd.StringVar(&c.QueryLookbackDelta, "query.lookback-delta", c.QueryLookbackDelta, "The delta difference allowed for retrieving metrics during expression evaluations.")

	cmd.StringVar(&c.QueryTimeout, "query.timeout", c.QueryTimeout, "Maximum time a query may take before being aborted.")

	cmd.StringVar(&c.QueryMaxConcurrency, "query.max-concurrency", c.QueryMaxConcurrency, "Maximum number of queries executed concurrently.")

	cmd.StringVar(&c.LogLevel, "log.level", c.LogLevel, "log level.")
}

func (c *Config) CompleteConfig() {
	// parse etcd urls line to array
	for _, url := range strings.Split(c.EtcdEndpointsLine, ",") {
		c.EtcdEndpoints = append(c.EtcdEndpoints, url)
	}

	if len(c.EtcdEndpoints) < 1 {
		logrus.Error("Must define the etcd endpoints by --etcd-endpoints")
		os.Exit(17)
	}

	// parse values from prometheus options to config
	ipPort := strings.TrimLeft(c.AdvertiseAddr, "shttp://")
	ipPortArr := strings.Split(ipPort, ":")
	c.BindIp = ipPortArr[0]
	port, err := strconv.Atoi(ipPortArr[1])
	if err == nil {
		c.Port = port
	}

	defaultOptions := "--log.level=%s --web.listen-address=%s --config.file=%s --storage.tsdb.path=%s --storage.tsdb.retention=%s"
	defaultOptions = fmt.Sprintf(defaultOptions, c.LogLevel, c.Web.ListenAddress, c.ConfigFile, c.LocalStoragePath, c.Tsdb.Retention)
	if c.Tsdb.NoLockfile {
		defaultOptions += " --storage.tsdb.no-lockfile"
	}
	if c.Web.EnableAdminAPI {
		defaultOptions += " --web.enable-admin-api"
	}
	if c.Web.EnableLifecycle {
		defaultOptions += " --web.enable-lifecycle"
	}

	args := strings.Split(defaultOptions, " ")
	c.StartArgs = append(c.StartArgs, os.Args[0])
	c.StartArgs = append(c.StartArgs, args...)

	level, err := logrus.ParseLevel(c.LogLevel)
	if err != nil {
		fmt.Println("ERROR set log level:", err)
		return
	}
	logrus.SetLevel(level)

	logrus.Info("Start with options: ", c)
}
