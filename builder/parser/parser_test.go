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

package parser

import (
	"testing"

	"github.com/goodrain/rainbond/builder/parser/types"
)

func TestParseImageName(t *testing.T) {
	image := ParseImageName("192.168.0.1:9090/asdasd/asdasd:asdad")
	t.Logf("string %s", image.String())
	t.Logf("domain %s", image.GetDomain())
	t.Logf("repostory %s", image.GetRepostory())
	t.Logf("sname %s", image.GetSimpleName())
	t.Logf("name %s", image.Name)
	t.Logf("tag %s", image.GetTag())
	image2 := ParseImageName("192.168.0.1/asdasd/name")
	t.Logf("string %s", image2.String())
	t.Logf("domain %s", image2.GetDomain())
	t.Logf("repostory %s", image2.GetRepostory())
	t.Logf("name %s", image2.GetSimpleName())
	t.Logf("tag %s", image2.GetTag())
	image3 := ParseImageName("mongo")
	t.Logf("string %s", image3.String())
	t.Logf("domain %s", image3.GetDomain())
	t.Logf("repostory %s", image3.GetRepostory())
	t.Logf("name %s", image3.GetSimpleName())
	t.Logf("tag %s", image3.GetTag())
	image4 := ParseImageName("abewang/foobar")
	t.Logf("string %s", image4.String())
	t.Logf("domain %s", image4.GetDomain())
	t.Logf("repostory %s", image4.GetRepostory())
	t.Logf("name %s", image4.GetSimpleName())
	t.Logf("tag %s", image4.GetTag())

	image5 := ParseImageName("")
	t.Logf("string %s", image5.String())
	t.Logf("domain %s", image5.GetDomain())
	t.Logf("repostory %s", image5.GetRepostory())
	t.Logf("name %s", image5.GetSimpleName())
	t.Logf("tag %s", image5.GetTag())
}
func TestDetermineDeployType(t *testing.T) {
	t.Log(DetermineDeployType(ParseImageName("barnett/zookeeper:3.2")))
	t.Log(DetermineDeployType(ParseImageName("elcolio/etcd:2.0.10")))
	t.Log(DetermineDeployType(ParseImageName("phpmyadmin")))
}

func TestReadmemory(t *testing.T) {
	testcases := []struct {
		mem string
		exp int
	}{
		{mem: "", exp: 512},
		{mem: "2Gi", exp: 2 * 1024},
		{mem: "2G", exp: 2 * 1024},
		{mem: "300Mi", exp: 300},
		{mem: "300m", exp: 300},
		{mem: "1024Ki", exp: 1},
		{mem: "1024k", exp: 1},
		{mem: "1024K", exp: 1},
		{mem: "1048576Bi", exp: 512},
		{mem: "abc", exp: 512},
	}
	for _, tc := range testcases {
		mem := readmemory(tc.mem)
		if mem != tc.exp {
			t.Errorf("mem: %s; Expected %d, but returned %d", tc.mem, tc.exp, mem)
		}
	}
}

func TestParseDockerRun(t *testing.T) {
	var dr = &DockerRunOrImageParse{
		ports:   make(map[int]*types.Port),
		volumes: make(map[string]*types.Volume),
		envs:    make(map[string]*types.Env),
	}
	drs := `
	docker run -p 9000:9000 --name minio1 \
  -e "MINIO_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE" \
  -e "MINIO_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" \
  -v /mnt/data:/data \
  -v /mnt/config:/root/.minio \
  minio/minio server /data`
	dr.ParseDockerun(drs)
	t.Log(dr.GetEnvs())
	t.Log(dr.GetPorts())
	t.Log(dr.GetVolumes())
	t.Log(dr.GetArgs())
	t.Log(dr.GetImage())
}
