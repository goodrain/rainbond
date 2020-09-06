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

package build

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/goodrain/rainbond/util"

	"github.com/docker/docker/client"

	"github.com/sirupsen/logrus"

	"github.com/docker/docker/api/types"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
)

var netcoreBuildDockerfile = "/src/build-app/netcore/Dockerfile.build"
var netcoreRuntimeDockerfile = "/src/build-app/netcore/Dockerfile.runtime"

// var netcoreBuildDockerfile = "/Users/qingguo/gopath/src/github.com/goodrain/rainbond/hack/contrib/docker/chaos/build-app/netcore/Dockerfile.build"
// var netcoreRuntimeDockerfile = "/Users/qingguo/gopath/src/github.com/goodrain/rainbond/hack/contrib/docker/chaos/build-app/netcore/Dockerfile.runtime"
var buildDockerfile []byte
var runDockerfile []byte

func netcoreBuilder() (Build, error) {
	if buildDockerfile == nil || runDockerfile == nil {
		build, err := ioutil.ReadFile(netcoreBuildDockerfile)
		if err != nil {
			return nil, err
		}
		runtime, err := ioutil.ReadFile(netcoreRuntimeDockerfile)
		if err != nil {
			return nil, err
		}
		buildDockerfile = build
		runDockerfile = runtime
	}
	return &netcoreBuild{}, nil
}

type netcoreBuild struct {
	imageName      string
	buildImageName string
	buildCacheDir  string
	sourceDir      string
	dockercli      *client.Client
	logger         event.Logger
	serviceID      string
}

func (d *netcoreBuild) Build(re *Request) (*Response, error) {
	d.dockercli = re.DockerClient
	d.logger = re.Logger
	d.serviceID = re.ServiceID
	defer d.clear()
	//write default Dockerfile for build
	if err := d.writeBuildDockerfile(re.SourceDir, re.BuildEnvs); err != nil {
		return nil, fmt.Errorf("write default build dockerfile error:%s", err.Error())
	}
	d.sourceDir = re.SourceDir
	d.imageName = CreateImageName(re.ServiceID, re.DeployVersion)
	d.buildImageName = d.imageName + "_build"
	//build code
	buildOptions := types.ImageBuildOptions{
		Tags:   []string{d.buildImageName},
		Remove: true,
	}
	if _, ok := re.BuildEnvs["NO_CACHE"]; ok {
		buildOptions.NoCache = true
	} else {
		buildOptions.NoCache = false
	}
	re.Logger.Info("start compiling the source code", map[string]string{"step": "builder-exector"})
	_, err := sources.ImageBuild(re.DockerClient, re.SourceDir, buildOptions, re.Logger, 20)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("build image %s failure, find log in rbd-chaos", d.buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("build image error: %s", err.Error())
		return nil, err
	}
	// check build image exist
	_, err = sources.ImageInspectWithRaw(re.DockerClient, d.buildImageName)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("build image %s failure, find log in rbd-chaos", d.buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("get image inspect error: %s", err.Error())
		return nil, err
	}
	// copy build output
	d.buildCacheDir = path.Join(re.CacheDir, re.DeployVersion)
	err = d.copyBuildOut(d.buildCacheDir, d.buildImageName)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("copy compilation package failed, find log in rbd-chaos"), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("copy build output file error: %s", err.Error())
		return nil, err
	}
	//write default runtime dockerfile
	if err := d.writeRunDockerfile(d.buildCacheDir, re.BuildEnvs); err != nil {
		return nil, fmt.Errorf("write default runtime dockerfile error:%s", err.Error())
	}
	//build runtime image
	runbuildOptions := types.ImageBuildOptions{
		Tags:   []string{d.imageName},
		Remove: true,
	}
	if _, ok := re.BuildEnvs["NO_CACHE"]; ok {
		runbuildOptions.NoCache = true
	} else {
		runbuildOptions.NoCache = false
	}
	_, err = sources.ImageBuild(re.DockerClient, d.buildCacheDir, runbuildOptions, re.Logger, 60)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("build image %s failure, find log in rbd-chaos", d.buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("build image error: %s", err.Error())
		return nil, err
	}
	// check build image exist
	_, err = sources.ImageInspectWithRaw(re.DockerClient, d.imageName)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("build image %s failure, find log in rbd-chaos", d.buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("get image inspect error: %s", err.Error())
		return nil, err
	}
	re.Logger.Info("build image success, start to push local image registry", map[string]string{"step": "builder-exector"})
	err = sources.ImagePush(re.DockerClient, d.imageName, builder.REGISTRYUSER, builder.REGISTRYPASS, re.Logger, 5)
	if err != nil {
		re.Logger.Error("push image to local image registry faliure, find log in rbd-chaos", map[string]string{"step": "builder-exector"})
		logrus.Errorf("push image error: %s", err.Error())
		return nil, err
	}
	re.Logger.Info("push image to push local image registry success", map[string]string{"step": "builder-exector"})
	if err := sources.ImageRemove(re.DockerClient, d.imageName); err != nil {
		logrus.Errorf("remove image %s failure %s", d.imageName, err.Error())
	}
	return d.createResponse(), nil
}
func (d *netcoreBuild) writeBuildDockerfile(sourceDir string, envs map[string]string) error {
	result := util.ParseVariable(string(buildDockerfile), envs)
	return ioutil.WriteFile(path.Join(sourceDir, "Dockerfile"), []byte(result), 0755)
}

func (d *netcoreBuild) writeRunDockerfile(sourceDir string, envs map[string]string) error {
	result := util.ParseVariable(string(runDockerfile), envs)
	return ioutil.WriteFile(path.Join(sourceDir, "Dockerfile"), []byte(result), 0755)
}

func (d *netcoreBuild) copyBuildOut(outDir string, sourceImage string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ds := sources.CreateDockerService(ctx, d.dockercli)
	cid, err := ds.CreateContainer(&sources.ContainerConfig{
		Metadata: &sources.ContainerMetadata{
			Name: d.serviceID + "_builder",
		},
		NetworkConfig: &sources.NetworkConfig{
			NetworkMode: "none",
		},
		Image: &sources.ImageSpec{
			Image: sourceImage,
		},
		Mounts: []*sources.Mount{
			&sources.Mount{
				ContainerPath: "/tmp/out",
				HostPath:      outDir,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create container copy file error %s", err.Error())
	}
	statuschan := ds.WaitExitOrRemoved(cid, true)
	if err := ds.StartContainer(cid); err != nil {
		return fmt.Errorf("start container copy file error %s", err.Error())
	}
	status := <-statuschan
	if status != 0 {
		return &ErrorBuild{Code: status}
	}
	return nil
}
func (d *netcoreBuild) createResponse() *Response {
	return &Response{
		MediumType: ImageMediumType,
		MediumPath: d.imageName,
	}
}

func (d *netcoreBuild) clear() {
	//os.RemoveAll(d.buildCacheDir)
	os.Remove(path.Join(d.sourceDir, "Dockerfile"))
}
