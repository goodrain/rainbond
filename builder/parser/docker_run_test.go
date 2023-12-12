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

package parser

import (
	"fmt"
	"testing"

	//"github.com/docker/docker/client"
	"github.com/docker/docker/client"
)

var dockerrun = `docker run -d -P -v /usr/share/ca-certificates/:/etc/ssl/certs -p 4001:4001 -p 2380:2380 -p 2379:2379 \
--name etcd quay.io/coreos/etcd:v2.3.8 \
-name etcd0 \
-advertise-client-urls http://0.0.0.0:2379,http://0.0.0.0:4001 \
-listen-client-urls http://0.0.0.0:2379,http://0.0.0.0:4001 \
-initial-advertise-peer-urls http://127.0.0.1:2380 \
-listen-peer-urls http://0.0.0.0:2380 \
-initial-cluster-token etcd-cluster-1 \
-initial-cluster etcd0=http://127.0.0.1:2380 \
-initial-cluster-state new`

var testCase = `docker run -d --restart=always --name powerjob-server -p 7700:7700 -p 10086:10086 -e TZ="Asia/Shanghai" -e JVMOPTIONS="" -e PARAMS="--spring.profiles.active=product --spring.datasource.core.jdbc-url=jdbc:postgresql://127.0.0.1:5432/powerjob-product?useUnicode=true&characterEncoding=UTF-8&serverTimezone=Asia/Shanghai --spring.datasource.core.username=admin --spring.datasource.core.password=d9e6c012 --oms.mongodb.enable=false --oms.mongodb.enable=false --spring.data.mongodb.uri=mongodb://127.0.0.1:27017/powerjob-product" -v ~/docker/powerjob-server:/root/powerjob-server -v ~/.m2:/root/.m2 tjqq/powerjob-server:latest`

func TestParse(t *testing.T) {
	dockerclient, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	p := CreateDockerRunOrImageParse("d", "", testCase, dockerclient, nil, "")
	p.ParseDockerun(testCase)
	fmt.Printf("ServiceInfo:%+v \n", p.GetServiceInfo())
}
