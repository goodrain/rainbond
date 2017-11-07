
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

package config

import (
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"os"
	"testing"

	"github.com/Sirupsen/logrus"

	"github.com/coreos/etcd/clientv3"
)

func init() {
	err := store.NewClient(&option.Conf{Etcd: clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	}})
	if err != nil {
		logrus.Error(err.Error())
		os.Exit(1)
	}
}
func TestGetDataCenterConfig(t *testing.T) {
	c := DataCenterConfig{options: &option.Conf{
		ConfigStorage: "/acp_node/configs",
	}}
	gc, err := c.GetDataCenterConfig()
	t.Log(gc.String())
	t.Fatal(err)
}
