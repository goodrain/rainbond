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

package client

import (
	"testing"
	"time"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/goodrain/rainbond/builder/exector"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/goodrain/rainbond/util/etcd"
	"github.com/twinj/uuid"

	context "golang.org/x/net/context"
)

func TestClient(t *testing.T) {
	c, err := NewMqClient(&etcd.ClientArgs{Endpoints: []string{"127.0.0.1:2379"}}, "127.0.0.1:2379")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100000; i++ {
		re, err := c.Enqueue(context.Background(), &pb.EnqueueRequest{
			Topic: "worker",
			Message: &pb.TaskMessage{
				TaskType:   "stop",
				CreateTime: time.Now().Format(time.RFC3339),
				TaskBody:   []byte(`{"tenant_id":"232bd923d3794b979974bb21b863608b","service_id":"37f6cc84da449882104687130e868196","deploy_version":"20170717163635","event_id":"system"}`),
				User:       "barnett",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Log(re)
		taskme, err := c.Dequeue(context.Background(), &pb.DequeueRequest{Topic: "worker"})
		if err != nil {
			t.Fatal(err)
		}
		t.Log(taskme)
		time.Sleep(time.Millisecond * 10)
	}

}

func TestClientScaling(t *testing.T) {
	t.SkipNow()
	client, err := NewMqClient(&etcd.ClientArgs{Endpoints: []string{"127.0.0.1:2379"}}, "127.0.0.1:2379")
	if err != nil {
		t.Fatal(err)
	}
	client.Enqueue(context.Background(), &pb.EnqueueRequest{
		Topic: "worker",
		Message: &pb.TaskMessage{
			TaskType:   "horizontal_scaling",
			CreateTime: time.Now().Format(time.RFC3339),
			TaskBody:   []byte(`{"tenant_id":"232bd923d3794b979974bb21b863608b","service_id":"59fbd0a74e7dfbf594fba0f8953593f8","replicas":1,"event_id":"system"}`),
			User:       "barnett",
		},
	})
}

func TestClientUpgrade(t *testing.T) {
	t.SkipNow()
	client, err := NewMqClient(&etcd.ClientArgs{Endpoints: []string{"127.0.0.1:2379"}}, "127.0.0.1:2379")
	if err != nil {
		t.Fatal(err)
	}
	client.Enqueue(context.Background(), &pb.EnqueueRequest{
		Topic: "worker",
		Message: &pb.TaskMessage{
			TaskType:   "rolling_upgrade",
			CreateTime: time.Now().Format(time.RFC3339),
			TaskBody:   []byte(`{"tenant_id":"232bd923d3794b979974bb21b863608b","service_id":"59fbd0a74e7dfbf594fba0f8953593f8","current_deploy_version":"20170725151249","new_deploy_version":"20170725151251","event_id":"system"}`),
			User:       "barnett",
		},
	})
}
func TestBuilder(t *testing.T) {
	t.SkipNow()
	c, err := NewMqClient(&etcd.ClientArgs{Endpoints: []string{"127.0.0.1:2379"}}, "127.0.0.1:2379")
	if err != nil {
		t.Fatal(err)
	}
	c.Enqueue(context.Background(), &pb.EnqueueRequest{
		Topic: "builder",
		Message: &pb.TaskMessage{
			TaskType:   "app_build",
			CreateTime: time.Now().Format(time.RFC3339),
			TaskBody:   []byte(`{"envs": {"DEBUG": "True"}, "expire": 180, "deploy_version": "20170905133413", "repo_url": "--branch master --depth 1 git@code.goodrain.com:goodrain/goodrain_web.git", "service_id": "f398048d1a2998b05e556330b05ec1aa", "event_id": "e0413f825cc740678e721fc5d5a9e825", "tenant_id": "b7584c080ad24fafaa812a7739174b50", "action": "upgrade", "operator": "lichao"}`),
			User:       "barnett",
		},
	})
}

func TestChaosTask(t *testing.T) {
	sci := exector.ServiceCheckInput{
		CheckUUID:  uuid.NewV4().String(),
		SourceType: "docker-run",
		SourceBody: "docker run -it nginx",
		EventID:    "system",
	}
	body, _ := ffjson.Marshal(sci)
	client, err := NewMqClient(&etcd.ClientArgs{Endpoints: []string{"127.0.0.1:2379"}}, "127.0.0.1:2379")
	if err != nil {
		t.Fatal(err)
	}
	client.Enqueue(context.Background(), &pb.EnqueueRequest{
		Topic: "builder",
		Message: &pb.TaskMessage{
			TaskType:   "service_check",
			CreateTime: time.Now().Format(time.RFC3339),
			TaskBody:   body,
			User:       "barnett",
		},
	})
}
