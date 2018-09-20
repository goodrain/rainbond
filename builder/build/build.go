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
	"regexp"
	"strings"

	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/event"

	"github.com/docker/engine-api/client"
)

func init() {
	buildcreaters = make(map[code.Lang]CreaterBuild)
	buildcreaters[code.Dockerfile] = dockerfileBuilder
	buildcreaters[code.Docker] = dockerfileBuilder
	buildcreaters[code.NetCore] = netcoreBuilder
	buildcreaters[code.JavaJar] = slugBuilder
	buildcreaters[code.JavaMaven] = slugBuilder
	buildcreaters[code.JaveWar] = slugBuilder
	buildcreaters[code.PHP] = slugBuilder
	buildcreaters[code.Python] = slugBuilder
	buildcreaters[code.Nodejs] = slugBuilder
	buildcreaters[code.Golang] = slugBuilder
}

var buildcreaters map[code.Lang]CreaterBuild

//Build app build pack
type Build interface {
	Build(*Request) (*Response, error)
}

//CreaterBuild CreaterBuild
type CreaterBuild func() (Build, error)

//MediumType Build output medium type
type MediumType string

//ImageMediumType image type
var ImageMediumType MediumType = "image"

//SlugMediumType slug type
var SlugMediumType MediumType = "slug"

//Response build result
type Response struct {
	MediumPath string
	MediumType MediumType
}

//Request build input
type Request struct {
	TenantID      string
	SourceDir     string
	CacheDir      string
	TGZDir        string
	RepositoryURL string
	Branch        string
	ServiceAlias  string
	ServiceID     string
	DeployVersion string
	Runtime       string
	ServerType    string
	Commit        Commit
	Lang          code.Lang
	BuildEnvs     map[string]string
	Logger        event.Logger
	DockerClient  *client.Client
}

//Commit Commit
type Commit struct {
	User    string
	Message string
	Hash    string
}

//GetBuild GetBuild
func GetBuild(lang code.Lang) (Build, error) {
	if fun, ok := buildcreaters[lang]; ok {
		return fun()
	}
	return slugBuilder()
}

//CreateImageName create image name
func CreateImageName(repoURL, serviceAlias, deployversion string) string {
	reg := regexp.MustCompile(`.*(?:\:|\/)([\w\-\.]+)/([\w\-\.]+)\.git`)
	rc := reg.FindSubmatch([]byte(repoURL))
	var name string
	if len(rc) == 3 {
		name = fmt.Sprintf("%s_%s_%s", serviceAlias, string(rc[1]), string(rc[2]))
	} else {
		name = fmt.Sprintf("%s_%s", serviceAlias, "rainbondbuild")
	}
	buildImageName := strings.ToLower(fmt.Sprintf("%s/%s:%s", builder.REGISTRYDOMAIN, name, deployversion))
	return buildImageName
}
