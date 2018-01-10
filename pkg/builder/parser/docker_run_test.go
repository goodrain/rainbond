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

package parser

import (
	"fmt"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/client"
)

var dockerrun = `
docker run --name my-custom-nginx-container \
-d -it \
-v=/host/path/nginx.conf:/etc/nginx/nginx.conf:ro \
-e=xxx=xxx \
--public 90:90 \
-m 10g \
 mysql

`

func TestParse(t *testing.T) {
	dockerclient, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	p := CreateDockerRunOrImageParse(dockerrun, dockerclient)
	if err := p.Parse(); err != nil {
		logrus.Errorf(err.Error())
	}
	fmt.Printf("Volume:%v \n", p.GetVolumes())
	fmt.Printf("Ports:%v \n", p.GetPorts())
	fmt.Printf("Envs:%v \n", p.GetEnvs())
	fmt.Printf("Image:%v \n", p.GetImage())
	fmt.Printf("Memory:%v \n", p.GetMemory())
	fmt.Printf("Args:%v \n", p.GetArgs())
}
