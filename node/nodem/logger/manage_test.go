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

package logger

import (
	"bufio"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/goodrain/rainbond/cmd/node-proxy/option"
)

func TestWatchConatainer(t *testing.T) {
	dc, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	cm := CreatContainerLogManage(&option.Conf{
		DockerCli: dc,
	})
	cm.Start()
	select {}
}

func TestGetConatainerLogger(t *testing.T) {
	dc, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	cm := CreatContainerLogManage(&option.Conf{
		DockerCli: dc,
	})

	stdout, _, err := cm.getContainerLogReader(cm.ctx, "9874f23cbfc8201571bc654955aad94124f256ccb37e10f914f80865d734a4c5")
	if err != nil {
		t.Fatal(err)
	}
	buffer := bufio.NewReader(stdout)
	for {
		line, _, err := buffer.ReadLine()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(string(line))
	}
}

func TestHostConfig(t *testing.T) {
	cj := new(types.ContainerJSON)
	if cj.ContainerJSONBase == nil || cj.HostConfig == nil || cj.HostConfig.LogConfig.Type == "" {
		fmt.Println("jsonBase is nil")
		cj.ContainerJSONBase = new(types.ContainerJSONBase)
	}
	if cj.ContainerJSONBase == nil || cj.HostConfig == nil || cj.HostConfig.LogConfig.Type == "" {
		fmt.Println("hostConfig is nil")
		cj.HostConfig = &container.HostConfig{}
	}
	if cj.ContainerJSONBase == nil || cj.HostConfig == nil || cj.HostConfig.LogConfig.Type == "" {
		fmt.Println("logconfig is nil won't panic")
		return
	}

}
