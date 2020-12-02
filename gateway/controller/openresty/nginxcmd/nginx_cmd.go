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

package nginxcmd

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	nginxBinary      = "nginx"
	defaultNginxConf = "/run/nginx/conf/nginx.conf"
	//ErrorCheck check config file failure
	ErrorCheck  = fmt.Errorf("error check config")
	updateCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "nginx",
		Subsystem: "",
		Name:      "update",
		Help:      "Number of nginx updates inside the gateway",
	})
	errUpdateCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "nginx",
		Subsystem: "",
		Name:      "update_err",
		Help:      "Number of nginx error updates inside the gateway",
	})
)

func init() {
	nginxBinary = path.Join(os.Getenv("OPENRESTY_HOME"), "/nginx/sbin/nginx")
	ngx := os.Getenv("NGINX_BINARY")
	if ngx != "" {
		nginxBinary = ngx
	}
}

//SetDefaultNginxConf set
func SetDefaultNginxConf(path string) {
	defaultNginxConf = path
}

//PromethesuScrape prometheus scrape
func PromethesuScrape(ch chan<- *prometheus.Desc) {
	updateCount.Describe(ch)
	errUpdateCount.Describe(ch)
}

//PrometheusCollect prometheus collect
func PrometheusCollect(ch chan<- prometheus.Metric) {
	updateCount.Collect(ch)
	errUpdateCount.Collect(ch)
}

//CreateNginxCommand create nginx command
func CreateNginxCommand(args ...string) *exec.Cmd {
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "-c", defaultNginxConf)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(nginxBinary, cmdArgs...)
	return cmd
}

//ExecNginxCommand exec nginx command
func ExecNginxCommand(args ...string) error {
	cmd := CreateNginxCommand(args...)
	if body, err := cmd.Output(); err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			logrus.Errorf("nginx exec failure:%s", string(eerr.Stderr))
		}
		if len(body) > 0 {
			logrus.Errorf("nginx exec failure:%s", string(body))
		}
		return err
	}
	return nil
}

//CheckConfig check nginx config file
func CheckConfig() error {
	if err := ExecNginxCommand("-t"); err != nil {
		return ErrorCheck
	}
	return nil
}

//Reload reload nginx config
func Reload() error {
	updateCount.Inc()
	if err := ExecNginxCommand("-s", "reload"); err != nil {
		errUpdateCount.Inc()
		return err
	}
	logrus.Infof("nginx config reload success")
	return nil
}
