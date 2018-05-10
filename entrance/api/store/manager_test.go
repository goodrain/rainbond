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

package store

import (
	"github.com/goodrain/rainbond/entrance/api/model"
	"github.com/goodrain/rainbond/cmd/entrance/option"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
)

func TestGetSourceList(t *testing.T) {
	manager, err := NewManager(option.Config{
		EtcdEndPoints: []string{"http://127.0.0.1:2379"},
		EtcdTimeout:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	manager.Register("domain", &model.Domain{})
	//对比反射与创建对象耗时
	start := time.Now()
	manager.New("source")
	logrus.Info(time.Now().UnixNano() - start.UnixNano())
	start = time.Now()
	_ = model.Domain{}
	logrus.Info(time.Now().UnixNano() - start.UnixNano())

	list, err := manager.GetSourceList("/store/tenants/barnett2/services/gr868196/domains", "domain")
	if err != nil {
		t.Fatal(err)
	}
	if list != nil {
		for _, domain := range list {
			logrus.Info(domain)
		}
	}

}
