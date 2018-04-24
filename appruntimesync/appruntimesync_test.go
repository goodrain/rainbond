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
package appruntimesync

import (
	"context"
	"testing"
	"time"

	"github.com/goodrain/rainbond/appruntimesync/client"

	"github.com/goodrain/rainbond/cmd/worker/option"
)

func TestCreateAppRuntimeSync(t *testing.T) {
	ars := CreateAppRuntimeSync(option.Config{
		KubeConfig: "../../test/admin.kubeconfig",
	})
	if err := ars.Start(); err != nil {
		t.Fatal(err)
	}
	defer ars.Stop()
	time.Sleep(time.Minute * 3)
}

func TestNewClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cli, err := client.NewClient(ctx, client.AppRuntimeSyncClientConf{EtcdEndpoints: []string{"127.0.0.1:2379"}})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second * 3)
	err = cli.SetStatus("abc", "running")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(cli.GetAllStatus())
	cli.CheckStatus("abc")
	cli.SetStatus("abc", "closed")
	t.Log(cli.GetStatus("abc"))
}
