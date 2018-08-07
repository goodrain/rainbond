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
	"testing"

	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/event"
)

func TestBuildNetCore(t *testing.T) {
	build, err := netcoreBuilder()
	if err != nil {
		t.Fatal(err)
	}
	dockerCli, _ := client.NewEnvClient()
	req := &Request{
		SourceDir:     "/Users/qingguo/goodrain/dotnet-docker/samples/aspnetapp/test",
		CacheDir:      "/Users/qingguo/goodrain/dotnet-docker/samples/aspnetapp/test/cache",
		RepositoryURL: "https://github.com/dotnet/dotnet-docker.git",
		ServiceAlias:  "gr123456",
		DeployVersion: "666666",
		Commit:        Commit{User: "barnett"},
		Lang:          code.NetCore,
		Logger:        event.GetTestLogger(),
		DockerClient:  dockerCli,
	}
	res, err := build.Build(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(*res)
}
