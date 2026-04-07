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
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/goodrain/rainbond/event"
)

// capability_id: rainbond.source-repo.clone
func TestGitClone(t *testing.T) {
	t.Skip("requires external git network access")
	start := time.Now()
	csi := CodeSourceInfo{
		RepositoryURL: "git@gitee.com:zhoujunhaogoodrain/webhook_test.git",
		Branch:        "master",
	}
	res, _, err := GitClone(csi, "/tmp/rainbonddoc3", event.GetTestLogger(), 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Take %d ms", time.Now().Unix()-start.Unix())
	commit, err := GetLastCommit(res)
	t.Logf("%+v %+v", commit, err)
}
// capability_id: rainbond.source-repo.clone-by-tag
func TestGitCloneByTag(t *testing.T) {
	t.Skip("requires external git network access")
	start := time.Now()
	csi := CodeSourceInfo{
		RepositoryURL: "https://github.com/goodrain/rainbond-ui.git",
		Branch:        "master",
	}
	res, _, err := GitClone(csi, "/tmp/rainbonddoc4", event.GetTestLogger(), 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Take %d ms", time.Now().Unix()-start.Unix())
	commit, err := GetLastCommit(res)
	t.Logf("%+v %+v", commit, err)
}

// capability_id: rainbond.source-repo.pull
func TestGitPull(t *testing.T) {
	t.Skip("requires external git network access")
	csi := CodeSourceInfo{
		RepositoryURL: "git@gitee.com:zhoujunhaogoodrain/webhook_test.git",
		Branch:        "master2",
	}
	res, _, err := GitPull(csi, "/tmp/master2", event.GetTestLogger(), 1)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := GetLastCommit(res)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%v", commit)
}

// capability_id: rainbond.source-repo.pull-or-clone
func TestGitPullOrClone(t *testing.T) {
	t.Skip("requires external git network access")
	csi := CodeSourceInfo{
		RepositoryURL: "git@gitee.com:zhoujunhaogoodrain/webhook_test.git",
	}
	res, _, err := GitCloneOrPull(csi, "/tmp/goodrainweb2", event.GetTestLogger(), 1)
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

// capability_id: rainbond.source-repo.cache-dir
func TestGetCodeCacheDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SOURCE_DIR", root)
	csi := CodeSourceInfo{
		RepositoryURL: "git@121.196.222.148:summersoft/yycx_push.git",
		Branch:        "test",
		TenantID:      "tenant-a",
		ServiceID:     "service-a",
	}
	dir := csi.GetCodeSourceDir()
	if !filepath.IsAbs(dir) {
		t.Fatalf("expected absolute cache dir, got %q", dir)
	}
	if filepath.Dir(filepath.Dir(dir)) != filepath.Join(root, "build") {
		t.Fatalf("unexpected cache dir root: %q", dir)
	}
}

// capability_id: rainbond.source-repo.show-url
func TestGetShowURL(t *testing.T) {
	got := getShowURL("https://zsl1526:79890ffc74014b34b49040d42b95d5af@github.com:9090/zsl1549/python-demo.git")
	want := "https://github.com:9090/zsl1549/python-demo.git"
	if got != want {
		t.Fatalf("getShowURL()=%q, want %q", got, want)
	}
}

// capability_id: rainbond.source-repo.git-ref-name
func TestGetBranch(t *testing.T) {
	if got := getBranch("main"); got != plumbing.ReferenceName("refs/heads/main") {
		t.Fatalf("unexpected branch ref: %q", got)
	}
	if got := getBranch("tag:v1.2.3"); got != plumbing.ReferenceName("refs/tags/v1.2.3") {
		t.Fatalf("unexpected tag ref: %q", got)
	}
}
