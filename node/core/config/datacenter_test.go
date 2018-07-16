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

package config

import (
	"strings"
	"testing"

	"github.com/goodrain/rainbond/node/api/model"
)

func init() {
	// err := store.NewClient(&option.Conf{Etcd: clientv3.Config{
	// 	Endpoints: []string{"127.0.0.1:2379"},
	// }})
	// if err != nil {
	// 	logrus.Error(err.Error())
	// 	os.Exit(1)
	// }
	// option.Config = &option.Conf{
	// 	ConfigStoragePath: "/rainbond/configs",
	// }
}
func TestGetDataCenterConfig(t *testing.T) {
	str := "asdadad,"
	t.Log(strings.Index(str, ","))
	c := GetDataCenterConfig()
	c.PutConfig(&model.ConfigUnit{
		Name:           strings.ToUpper("ARRAY"),
		Value:          []string{"121211212", "", "sadasd"},
		ValueType:      "array",
		IsConfigurable: false,
	})
	gc, err := c.GetDataCenterConfig()
	t.Log(strings.Join(gc.Get("ARRAY").Value.([]string), ","))
	t.Fatal(err)
}
