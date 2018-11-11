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
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/spf13/pflag"
)

type GWServer struct {
	Config
	LogLevel string
}

func NewGWServer() *GWServer {
	return &GWServer{}
}

//Config contains all configuration
type Config struct {
	Nginx       model.Nginx
	Http        model.Http
	K8SConfPath string
	Namespace string
}

// AddFlags adds flags
func (g *GWServer) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&g.LogLevel, "log-level", "debug", "the gateway log level")
	// TODO change kube-conf
	fs.StringVar(&g.K8SConfPath, "kube-conf", "/Users/abe/Documents/admin.kubeconfig", "absolute path to the kubeconfig file")
	fs.StringVar(&g.Namespace, "namespace", "gateway", "namespace")
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
