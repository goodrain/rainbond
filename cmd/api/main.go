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

// Rainbond datacenter api binary
package main

import (
	"context"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/pkg/component"
	"github.com/goodrain/rainbond/pkg/rainbond"
	"github.com/sirupsen/logrus"
	"os"

	"github.com/goodrain/rainbond/cmd"
	"github.com/goodrain/rainbond/cmd/api/option"

	"github.com/spf13/pflag"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		cmd.ShowVersion("api")
	}
	s := option.NewAPIServer()
	s.AddFlags(pflag.CommandLine)
	pflag.Parse()
	s.SetLog()
	err := rainbond.New(context.Background(), &configs.Config{
		AppName:   "rbd-api",
		APIConfig: s.Config,
	}).Registry(component.Database()).
		Registry(component.Grpc()).
		Registry(component.Event()).
		Registry(component.K8sClient()).
		Registry(component.HubRegistry()).
		Registry(component.Proxy()).
		Registry(component.Etcd()).
		Registry(component.Handler()).
		Registry(component.Router()).
		Start()
	if err != nil {
		logrus.Errorf("start rbd-api error %s", err.Error())
	}
}
