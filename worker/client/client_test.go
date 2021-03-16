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

package client

import (
	"context"
	"testing"
	"time"

	"github.com/goodrain/rainbond/worker/server/pb"
)

func TestGetAppStatus(t *testing.T) {
	client, err := NewClient(context.Background(), AppRuntimeSyncClientConf{
		EtcdEndpoints: []string{"192.168.3.252:2379"},
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Second)
	status, err := client.GetAppStatus(context.Background(), &pb.AppStatusReq{
		AppId: "43eaae441859eda35b02075d37d83589",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(status)
}

func TestGetPodDetail(t *testing.T) {
	client, err := NewClient(context.Background(), AppRuntimeSyncClientConf{
		EtcdEndpoints: []string{"127.0.0.1:2379"},
	})
	if err != nil {
		t.Fatalf("error creating grpc client: %v", err)
	}

	time.Sleep(3 * time.Second)

	sid := "bc2e153243e4917acaf0c6088789fa12"
	podname := "bc2e153243e4917acaf0c6088789fa12-deployment-6cc445949b-2dh25"
	detail, err := client.GetPodDetail(sid, podname)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	t.Logf("pod detail: %+v", detail)
}
