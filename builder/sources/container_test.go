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

package sources

import (
	"log"
	"testing"

	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

func createService() *DockerService {
	client, err := client.NewEnvClient()
	if err != nil {
		log.Fatal(err)
	}
	service := CreateDockerService(context.Background(), client)
	return service
}
func TestCreateContainer(t *testing.T) {
	service := createService()
	containerID, err := service.CreateContainer(&ContainerConfig{
		Metadata: &ContainerMetadata{
			Name: "test",
		},
		Image: &ImageSpec{
			Image: "nginx",
		},
		Args: []string{""},
		Devices: []*Device{
			&Device{
				ContainerPath: "/data",
				HostPath:      "/tmp/test",
			},
		},
		Mounts: []*Mount{
			&Mount{
				ContainerPath: "/data2",
				HostPath:      "/tmp/test2",
				Readonly:      false,
			},
		},
		Envs: []*KeyValue{
			&KeyValue{Key: "ABC", Value: "ASDA ASDASD ASDASD \"ASDAASD"},
		},
		Stdin: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(containerID)
}

func TestStartContainer(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	service := createService()
	containerID, err := service.CreateContainer(&ContainerConfig{
		Metadata: &ContainerMetadata{
			Name: "test",
		},
		Image: &ImageSpec{
			Image: "containertest",
		},
		Mounts: []*Mount{
			&Mount{
				ContainerPath: "/data2",
				HostPath:      "/tmp/test2",
				Readonly:      false,
			},
		},
		Envs: []*KeyValue{
			&KeyValue{Key: "ABC", Value: "ASDA ASDASD ASDASD \"ASDAASD"},
		},
		Stdin: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	errchan := make(chan error, 1)
	//start the container
	if err := service.StartContainer(containerID); err != nil {
		<-errchan
		t.Fatal(err)
	}
	if errchan != nil {
		if err := <-errchan; err != nil {
			logrus.Debugf("Error hijack: %s", err)
			t.Fatal(err)
		}
	}
	t.Log(containerID)
}
