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
	"path"

	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
)

var dockerfileTmpl = `
FROM mcr.microsoft.com/dotnet/sdk:${DOTNET_SDK_VERSION:2.1-alpine} AS builder
WORKDIR /app

# copy csproj and restore as distinct layers
COPY . .
RUN ${DOTNET_RESTORE_PRE} && ${DOTNET_RESTORE:dotnet restore} && dotnet publish -c Release -o /out

FROM mcr.microsoft.com/dotnet/aspnet:${DOTNET_RUNTIME_VERSION:2.1-alpine}
WORKDIR /app
COPY --from=builder /out/ .
CMD ["dotnet"]
`

type netcoreBuild struct {
	imageName      string
	buildImageName string
	sourceDir      string
	logger         event.Logger
	serviceID      string
	imageClient    sources.ImageClient
}

func netcoreBuilder() (Build, error) {
	return &netcoreBuild{}, nil
}

func (d *netcoreBuild) Build(re *Request) (*Response, error) {
	defer d.clear()
	d.logger = re.Logger
	d.serviceID = re.ServiceID
	d.sourceDir = re.SourceDir
	d.imageName = CreateImageName(re.ServiceID, re.DeployVersion)
	d.imageClient = re.ImageClient

	re.Logger.Info("start compiling the source code", map[string]string{"step": "builder-exector"})
	// write dockerfile
	if err := d.writeDockerfile(d.sourceDir, re.BuildEnvs); err != nil {
		return nil, fmt.Errorf("write default dockerfile error:%s", err.Error())
	}
	// build image
	err := sources.ImageBuild(re.Arch, d.sourceDir, re.CachePVCName, re.CacheMode, re.RbdNamespace, re.ServiceID, re.DeployVersion, re.Logger, "nc-build", "", re.KanikoImage, re.KanikoArgs)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("build image %s failure, find log in rbd-chaos", d.buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("build image error: %s", err.Error())
		return nil, err
	}
	re.Logger.Info("push image to push local image registry success", map[string]string{"step": "builder-exector"})
	if err := d.imageClient.ImageRemove(d.imageName); err != nil {
		logrus.Errorf("remove image %s failure %s", d.imageName, err.Error())
	}
	return d.createResponse(), nil
}

func (d *netcoreBuild) writeDockerfile(sourceDir string, envs map[string]string) error {
	dockerfile := util.ParseVariable(dockerfileTmpl, envs)
	dfpath := path.Join(sourceDir, "Dockerfile")
	logrus.Debugf("dest: %s; write dockerfile: %s", dfpath, dockerfile)
	return ioutil.WriteFile(dfpath, []byte(dockerfile), 0755)
}

func (d *netcoreBuild) createResponse() *Response {
	return &Response{
		MediumType: ImageMediumType,
		MediumPath: d.imageName,
	}
}

func (d *netcoreBuild) clear() {
	os.Remove(path.Join(d.sourceDir, "Dockerfile"))
}
