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
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/event"

	"github.com/docker/engine-api/client"
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

func TestWaitExitOrRemoved(t *testing.T) {
	service := createService()
	exist := service.WaitExitOrRemoved("ebe308d2e69be555d492f3bd7960c908b9915e87b278fe661838b3e4b1a9196b", false)
	t.Log(<-exist)
}

func TestStartBuildContainer(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	service := createService()
	containerID, err := service.CreateContainer(&ContainerConfig{
		Metadata: &ContainerMetadata{
			Name: "builder__11",
		},
		Image: &ImageSpec{
			Image: "containertest",
		},
		Mounts: []*Mount{
			&Mount{
				ContainerPath: "/tmp/cache",
				HostPath:      "/tmp/buildtest/cache",
				Readonly:      false,
			},
		},
		Envs: []*KeyValue{
			&KeyValue{Key: "LANG", Value: "static"},
		},
		Stdin:        true,
		StdinOnce:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		NetworkConfig: &NetworkConfig{
			NetworkMode: "host",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	errchan := make(chan error, 1)
	tarfile, err := os.OpenFile("/Users/qingguo/gopath/src/github.com/goodrain/rainbond/test/testcontainer/test.tar", os.O_RDONLY, 0755)
	if err != nil {
		t.Fatal(err)
	}
	defer tarfile.Close()
	logger := event.GetTestLogger()
	writer := logger.GetWriter("builder", "debug")
	close, err := service.AttachContainer(containerID, true, true, true, tarfile, writer, writer, &errchan)
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	statuschan := service.WaitExitOrRemoved(containerID, false)
	//start the container
	if err := service.StartContainer(containerID); err != nil {
		<-errchan
		t.Fatal(err)
	}
	logrus.Infof("start container complete")
	if errchan != nil {
		if err := <-errchan; err != nil {
			logrus.Debugf("Error hijack: %s", err)
			t.Fatal(err)
		}
	}
	logrus.Infof("watch status chan")
	t.Log(<-statuschan)
	t.Log(containerID)

}
