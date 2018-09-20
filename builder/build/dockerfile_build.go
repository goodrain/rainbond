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
	"path"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/types"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
)

func dockerfileBuilder() (Build, error) {
	return &dockerfileBuild{}, nil
}

type dockerfileBuild struct {
}

func (d *dockerfileBuild) Build(re *Request) (*Response, error) {
	filepath := path.Join(re.SourceDir, "Dockerfile")
	re.Logger.Info("Start parse Dockerfile", map[string]string{"step": "builder-exector"})
	_, err := sources.ParseFile(filepath)
	if err != nil {
		logrus.Error("parse dockerfile error.", err.Error())
		re.Logger.Error(fmt.Sprintf("Parse dockerfile error"), map[string]string{"step": "builder-exector"})
		return nil, err
	}
	buildImageName := CreateImageName(re.RepositoryURL, re.ServiceAlias, re.DeployVersion)
	args := make(map[string]string, 5)
	for k, v := range re.BuildEnvs {
		if ks := strings.Split(k, "ARG_"); len(ks) > 1 {
			args[ks[1]] = v
		}
	}
	buildOptions := types.ImageBuildOptions{
		Tags:      []string{buildImageName},
		Remove:    true,
		BuildArgs: args,
	}
	if _, ok := re.BuildEnvs["NO_CACHE"]; ok {
		buildOptions.NoCache = true
	} else {
		buildOptions.NoCache = false
	}
	re.Logger.Info("Start build image from dockerfile", map[string]string{"step": "builder-exector"})
	err = sources.ImageBuild(re.DockerClient, re.SourceDir, buildOptions, re.Logger, 30)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("build image %s failure", buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("build image error: %s", err.Error())
		return nil, err
	}
	// check image exist
	_, err = sources.ImageInspectWithRaw(re.DockerClient, buildImageName)
	if err != nil {
		re.Logger.Error(fmt.Sprintf("Build image %s failure,view build logs", buildImageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("get image inspect error: %s", err.Error())
		return nil, err
	}
	re.Logger.Info("The image build is successful and starts pushing the image to the repository", map[string]string{"step": "builder-exector"})
	err = sources.ImagePush(re.DockerClient, buildImageName, builder.REGISTRYUSER, builder.REGISTRYPASS, re.Logger, 20)
	if err != nil {
		re.Logger.Error("Push image failure", map[string]string{"step": "builder-exector"})
		logrus.Errorf("push image error: %s", err.Error())
		return nil, err
	}
	re.Logger.Info("The image is pushed to the warehouse successfully", map[string]string{"step": "builder-exector"})

	return &Response{
		MediumPath: buildImageName,
		MediumType: ImageMediumType,
	}, nil
}
