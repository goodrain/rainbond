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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/Sirupsen/logrus"

	"github.com/docker/engine-api/types"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
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
}

func (d *netcoreBuild) Build(re *Request) (*Response, error) {
	defer d.clear()
	//write default Dockerfile for build
	if err := d.writeBuildDockerfile(re.SourceDir); err != nil {
		return nil, fmt.Errorf("write default build dockerfile error:%s", err.Error())
	}
	d.sourceDir = re.SourceDir
	d.imageName = CreateImageName(re.RepositoryURL, re.ServiceAlias, re.DeployVersion)
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
	re.Logger.Info("开始编译源码", map[string]string{"step": "builder-exector"})
	err := sources.ImageBuild(re.DockerClient, re.SourceDir, buildOptions, re.Logger, 20)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("构造编译镜像%s失败", d.buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("build image error: %s", err.Error())
		return nil, err
	}
	// check build image exist
	_, err = sources.ImageInspectWithRaw(re.DockerClient, d.buildImageName)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("构造镜像%s失败,请查看Debug日志", d.buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("get image inspect error: %s", err.Error())
		return nil, err
	}
	// copy build output
	d.buildCacheDir = path.Join(re.CacheDir, re.DeployVersion)
	err = d.copyBuildOut(d.buildCacheDir, d.buildImageName)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("复制编译包失败"), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("copy build output file error: %s", err.Error())
		return nil, err
	}
	//write default runtime dockerfile
	if err := d.writeRunDockerfile(d.buildCacheDir); err != nil {
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
	err = sources.ImageBuild(re.DockerClient, d.buildCacheDir, runbuildOptions, re.Logger, 20)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("构造应用镜像%s失败", d.buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("build image error: %s", err.Error())
		return nil, err
	}
	// check build image exist
	_, err = sources.ImageInspectWithRaw(re.DockerClient, d.imageName)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("构造镜像%s失败,请查看Debug日志", d.buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("get image inspect error: %s", err.Error())
		return nil, err
	}
	re.Logger.Info("镜像构建成功，开始推送镜像至仓库", map[string]string{"step": "builder-exector"})
	err = sources.ImagePush(re.DockerClient, d.imageName, builder.REGISTRYUSER, builder.REGISTRYPASS, re.Logger, 5)
	if err != nil {
		re.Logger.Error("推送镜像失败", map[string]string{"step": "builder-exector"})
		logrus.Errorf("push image error: %s", err.Error())
		return nil, err
	}
	re.Logger.Info("镜像推送镜像至仓库成功", map[string]string{"step": "builder-exector"})
	return d.createResponse(), nil
}
func (d *netcoreBuild) writeBuildDockerfile(sourceDir string) error {
	return ioutil.WriteFile(path.Join(sourceDir, "Dockerfile"), buildDockerfile, 0755)
}

func (d *netcoreBuild) writeRunDockerfile(sourceDir string) error {
	return ioutil.WriteFile(path.Join(sourceDir, "Dockerfile"), runDockerfile, 0755)
}

func (d *netcoreBuild) copyBuildOut(outDir string, sourceImage string) error {
	dockerbin, err := exec.LookPath("docker")
	if err != nil {
		return err
	}
	cmd := exec.Command(dockerbin, "run", "-t", "-v", outDir+":/tmp/out", "--rm", sourceImage)
	if err := cmd.Run(); err != nil {
		return err
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
