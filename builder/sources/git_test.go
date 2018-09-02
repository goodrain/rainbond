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
	"io"
	"testing"
	"time"

	"github.com/goodrain/rainbond/event"
)

func init() {
	event.NewManager(event.EventConfig{
		DiscoverAddress: []string{"172.17.0.1:2379"},
	})
}
func TestGitClone(t *testing.T) {
	start := time.Now()
	csi := CodeSourceInfo{
		RepositoryURL: "git@gitee.com:zhoujunhaogoodrain/webhook_test.git",
		Branch:        "master",
	}
	//logger := event.GetManager().GetLogger("system")
	res, err := GitClone(csi, "/tmp/rainbonddoc3", event.GetTestLogger(), 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Take %d ms", time.Now().Unix()-start.Unix())
	commit, err := GetLastCommit(res)
	t.Logf("%+v %+v", commit, err)
}
func TestGitCloneByTag(t *testing.T) {
	start := time.Now()
	csi := CodeSourceInfo{
		RepositoryURL: "https://github.com/goodrain/rainbond-install.git",
		Branch:        "tag:v3.5.1",
	}
	//logger := event.GetManager().GetLogger("system")
	res, err := GitClone(csi, "/tmp/rainbonddoc4", event.GetTestLogger(), 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Take %d ms", time.Now().Unix()-start.Unix())
	commit, err := GetLastCommit(res)
	t.Logf("%+v %+v", commit, err)
}

func TestGitPull(t *testing.T) {
	csi := CodeSourceInfo{
		RepositoryURL: "git@gitee.com:zhoujunhaogoodrain/webhook_test.git",
		Branch:        "master2",
	}
	//logger := event.GetManager().GetLogger("system")
	res, err := GitPull(csi, "/tmp/master2", event.GetTestLogger(), 1)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := GetLastCommit(res)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%v", commit)
}

func TestGitPullOrClone(t *testing.T) {
	csi := CodeSourceInfo{
		RepositoryURL: "git@gitee.com:zhoujunhaogoodrain/webhook_test.git",
	}
	//logger := event.GetManager().GetLogger("system")
	res, err := GitCloneOrPull(csi, "/tmp/goodrainweb2", event.GetTestLogger(), 1)
	if err != nil {
		t.Fatal(err)
	}
	//get last commit
	commit, err := GetLastCommit(res)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	t.Logf("%+v", commit)
}

func TestGetCodeCacheDir(t *testing.T) {
	csi := CodeSourceInfo{
		RepositoryURL: "git@121.196.222.148:summersoft/yycx_push.git",
		Branch:        "test",
	}
	t.Log(csi.GetCodeSourceDir())
}
