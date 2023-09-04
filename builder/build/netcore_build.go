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
	"github.com/goodrain/rainbond/builder/parser/code"
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

var nodeJSStaticDockerfileTmpl = `
FROM node:${RUNTIMES:20}-bullseye-slim AS builder
COPY . /app
WORKDIR /app
RUN ${PACKAGE_TOOL:npm} config set registry ${NPM_REGISTRY:https://registry.npmmirror.com} && ${PACKAGE_TOOL:npm} install && ${NODE_BUILD_CMD:npm run build}

FROM nginx:alpine
COPY nginx.k8s.conf /etc/nginx/conf.d/default.conf

COPY --from=builder /app/${DIST_DIR:dist} /opt/${DIST_DIR:dist}
`

var nginxTemplate = `
server {
    listen       5000;
    
    location / {
        root /opt/${DIST_DIR:dist};
    }
}
`

type customDockerfileBuild struct {
	imageName      string
	buildImageName string
	sourceDir      string
	logger         event.Logger
	serviceID      string
	imageClient    sources.ImageClient
}

func customDockerBuilder() (Build, error) {
	return &customDockerfileBuild{}, nil
}

func (d *customDockerfileBuild) Build(re *Request) (*Response, error) {
	defer d.clear()
	d.logger = re.Logger
	d.serviceID = re.ServiceID
	d.sourceDir = re.SourceDir
	d.imageName = CreateImageName(re.ServiceID, re.DeployVersion)
	d.imageClient = re.ImageClient

	re.Logger.Info("start compiling the source code", map[string]string{"step": "builder-exector"})
	// write dockerfile
	if err := d.writeDockerfile(d.sourceDir, re.BuildEnvs, re.Lang); err != nil {
		return nil, fmt.Errorf("write default dockerfile error:%s", err.Error())
	}
	// build image
	err := sources.ImageBuild(re.Arch, d.sourceDir, re.CachePVCName, re.CacheMode, re.RbdNamespace, re.ServiceID, re.DeployVersion, re.Logger, "nc-build", "", re.BuildKitImage, re.BuildKitArgs, re.BuildKitCache, re.KubeClient)
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

func (d *customDockerfileBuild) writeDockerfile(sourceDir string, envs map[string]string, lang code.Lang) error {
	dockerfile := util.ParseVariable(dockerfileTmpl, envs)
	if lang == "NodeJSStatic" && envs["MODE"] == "DOCKERFILE" {
		if envs["NODE_BUILD_CMD"] == "" {
			envs["NODE_BUILD_CMD"] = envs["PACKAGE_TOOL"] + " run build"
		}
		dockerfile = util.ParseVariable(nodeJSStaticDockerfileTmpl, envs)
		dPath := path.Join(sourceDir, "nginx.k8s.conf")
		nginxFile := util.ParseVariable(nginxTemplate, envs)
		err := ioutil.WriteFile(dPath, []byte(nginxFile), 0755)
		if err != nil {
			return err
		}
	}
	dfpath := path.Join(sourceDir, "Dockerfile")
	logrus.Debugf("dest: %s; write dockerfile: %s", dfpath, dockerfile)
	return ioutil.WriteFile(dfpath, []byte(dockerfile), 0755)
}

func (d *customDockerfileBuild) createResponse() *Response {
	return &Response{
		MediumType: ImageMediumType,
		MediumPath: d.imageName,
	}
}

func (d *customDockerfileBuild) clear() {
	os.Remove(path.Join(d.sourceDir, "Dockerfile"))
}
