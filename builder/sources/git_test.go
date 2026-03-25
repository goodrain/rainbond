// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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
	"testing"
)

func TestGitClone(t *testing.T) {
	t.Skip("integration test depends on external git repositories")
}
func TestGitCloneByTag(t *testing.T) {
	t.Skip("integration test depends on external git repositories")
}

func TestGitPull(t *testing.T) {
	t.Skip("integration test depends on external git repositories")
}

func TestGitPullOrClone(t *testing.T) {
	t.Skip("integration test depends on external git repositories")
}

func TestGetCodeCacheDir(t *testing.T) {
	csi := CodeSourceInfo{
		RepositoryURL: "git@121.196.222.148:summersoft/yycx_push.git",
		Branch:        "test",
	}
	t.Log(csi.GetCodeSourceDir())
}

func TestGetShowURL(t *testing.T) {
	t.Log(getShowURL("https://zsl1526:79890ffc74014b34b49040d42b95d5af@github.com:9090/zsl1549/python-demo.git"))
}
