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

package zeus

import (
	"github.com/goodrain/rainbond/entrance/core/object"
	"github.com/goodrain/rainbond/entrance/plugin"
	"fmt"
	"testing"

	"golang.org/x/net/context"
)

func TestAddPool(t *testing.T) {
	p, err := New(plugin.Context{
		Option: map[string]string{
			"user":     "admin",
			"password": "gr123465!",
			"urls":     "https://test.goodrain.com:9070",
		},
		Ctx: context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = p.AddPool(&object.PoolObject{
		Name: "tenant@xxx.Pool",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeletePool(t *testing.T) {
	p, err := New(plugin.Context{
		Option: map[string]string{
			"user":     "admin",
			"password": "gr123465!",
			"urls":     "https://test.goodrain.com:9070",
		},
		Ctx: context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = p.DeletePool(&object.PoolObject{
		Name: "tenant@xxx.Pool",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVSProperties(t *testing.T) {
	p := VSProperties{
		SSL: VSssl{},
	}
	b := GetJSON(p)
	fmt.Println("json:" + string(b))
}

func TestPoolCreate(t *testing.T) {
	var nodesTable []*ZeusNode
	poolBasic := PoolBasic{
		Note:       "",
		NodesTable: nodesTable,
		Monitors:   []string{"Connect"},
	}
	zeusSource := Source{
		Properties: PoolProperties{
			Basic: poolBasic,
		},
	}
	body, err := zeusSource.GetJSON()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(body))
}
