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
	image3 := ParseImageName("barnett/name:tag")
	t.Logf("string %s", image3.String())
	t.Logf("domain %s", image3.GetDomain())
	t.Logf("repostory %s", image3.GetRepostory())
	t.Logf("name %s", image3.GetSimpleName())
	t.Logf("tag %s", image3.GetTag())
}
func TestDetermineDeployType(t *testing.T) {
	t.Log(DetermineDeployType(ParseImageName("barnett/zookeeper:3.2")))
	t.Log(DetermineDeployType(ParseImageName("elcolio/etcd:2.0.10")))
	t.Log(DetermineDeployType(ParseImageName("phpmyadmin")))
}
