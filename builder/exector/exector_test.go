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

package exector

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/docker/docker/client"
	"k8s.io/client-go/kubernetes"

	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/cmd/chaos/option"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"

	mqclient "github.com/goodrain/rainbond/mq/client"
	etcdutil "github.com/goodrain/rainbond/util/etcd"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
)

func Test_exectorManager_buildFromSourceCode(t *testing.T) {
	conf := option.Config{
		EtcdEndPoints:       []string{"192.168.2.203:2379"},
		MQAPI:               "192.168.2.203:6300",
		EventLogServers:     []string{"192.168.2.203:6366"},
		RbdRepoName:         "rbd-dns",
		RbdNamespace:        "rbd-system",
		MysqlConnectionInfo: "EeM2oc:lee7OhQu@tcp(192.168.2.203:3306)/region",
	}
	etcdArgs := etcdutil.ClientArgs{Endpoints: conf.EtcdEndPoints}
	event.NewManager(event.EventConfig{
		EventLogServers: conf.EventLogServers,
	})
	restConfig, err := k8sutil.NewRestConfig("/Users/fanyangyang/Documents/company/goodrain/admin.kubeconfig")
	if err != nil {
		t.Fatal(err)
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints:   conf.EtcdEndPoints,
		DialTimeout: 10 * time.Second,
	})
	var maxConcurrentTask int
	if conf.MaxTasks == 0 {
		maxConcurrentTask = runtime.NumCPU() * 2
	} else {
		maxConcurrentTask = conf.MaxTasks
	}
	mqClient, err := mqclient.NewMqClient(&etcdArgs, conf.MQAPI)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	e := &exectorManager{
		DockerClient:      dockerClient,
		KubeClient:        kubeClient,
		EtcdCli:           etcdCli,
		tasks:             make(chan *pb.TaskMessage, maxConcurrentTask),
		maxConcurrentTask: maxConcurrentTask,
		mqClient:          mqClient,
		ctx:               ctx,
		cancel:            cancel,
		cfg:               conf,
	}
	taskBodym := make(map[string]interface{})
	taskBodym["repo_url"] = "https://github.com/goodrain/java-maven-demo.git"
	taskBodym["branch"] = "master"
	taskBodym["tenant_id"] = "5d7bd886e6dc4425bb6c2ac5fc9fa593"
	taskBodym["service_id"] = "4eaa41ccf145b8e43a6aeb1a5efeab53"
	taskBodym["deploy_version"] = "20200115193617"
	taskBodym["lang"] = code.JavaMaven
	taskBodym["event_id"] = "0000"
	taskBodym["envs"] = map[string]string{}

	taskBody, _ := json.Marshal(taskBodym)
	task := pb.TaskMessage{
		TaskType: "build_from_source_code",
		TaskBody: taskBody,
	}
	i := NewSouceCodeBuildItem(task.TaskBody)
	if err := i.Run(30 * time.Second); err != nil {
		t.Fatal(err)
	}
	e.buildFromSourceCode(&task)
}
