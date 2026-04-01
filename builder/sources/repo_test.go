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
	"path/filepath"
	"strings"
	"testing"
)

// capability_id: rainbond.source-repo.build-info
func TestCreateRepostoryBuildInfo(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SOURCE_DIR", root)

	info, err := CreateRepostoryBuildInfo(
		"ssh://git@gr5042d6.7804f67d.ali-sh-s1.goodrain.net:20905/root/private2018.git?dir=abc",
		"git",
		"master",
		"tenant-a",
		"service-a",
	)
	if err != nil {
		t.Fatal(err)
	}
	if info.RepostoryURL != "ssh://git@gr5042d6.7804f67d.ali-sh-s1.goodrain.net:20905/root/private2018.git" {
		t.Fatalf("unexpected repository url: %q", info.RepostoryURL)
	}
	if info.BuildPath != "abc" {
		t.Fatalf("unexpected build path: %q", info.BuildPath)
	}
	if !strings.HasPrefix(info.CodeHome, filepath.Join(root, "build", "tenant-a")) {
		t.Fatalf("unexpected code home: %q", info.CodeHome)
	}
}

// capability_id: rainbond.source-repo.temp-build-info
func TestCreateTempRepostoryBuildInfo(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SOURCE_DIR", root)

	infoA, err := CreateTempRepostoryBuildInfo(
		"ssh://git@gr5042d6.7804f67d.ali-sh-s1.goodrain.net:20905/root/private2018.git?dir=abc",
		"git",
		"master",
		"tenant-a",
		"service-a",
	)
	if err != nil {
		t.Fatal(err)
	}
	infoB, err := CreateTempRepostoryBuildInfo(
		"ssh://git@gr5042d6.7804f67d.ali-sh-s1.goodrain.net:20905/root/private2018.git?dir=abc",
		"git",
		"master",
		"tenant-a",
		"service-a",
	)
	if err != nil {
		t.Fatal(err)
	}

	if infoA.RepostoryURL != "ssh://git@gr5042d6.7804f67d.ali-sh-s1.goodrain.net:20905/root/private2018.git" {
		t.Fatalf("unexpected repository url: %q", infoA.RepostoryURL)
	}
	if infoA.BuildPath != "abc" {
		t.Fatalf("unexpected build path: %q", infoA.BuildPath)
	}
	expectedParent := filepath.Join(root, "build", "tenant-a")
	if filepath.Dir(infoA.CodeHome) != expectedParent {
		t.Fatalf("unexpected temp dir parent: %q", infoA.CodeHome)
	}
	if filepath.Dir(infoB.CodeHome) != expectedParent {
		t.Fatalf("unexpected temp dir parent: %q", infoB.CodeHome)
	}
	if infoA.CodeHome == infoB.CodeHome {
		t.Fatalf("expected unique temp code homes, got %q", infoA.CodeHome)
	}
}

// capability_id: rainbond.source-repo.temp-build-info-pkg
func TestCreateTempRepostoryBuildInfoForPkg(t *testing.T) {
	info, err := CreateTempRepostoryBuildInfo(
		"/grdata/package_build/components/service-a/events/event-a",
		"pkg",
		"",
		"tenant-a",
		"service-a",
	)
	if err != nil {
		t.Fatal(err)
	}
	if info.CodeHome != "/grdata/package_build/components/service-a/events/event-a" {
		t.Fatalf("unexpected pkg code home: %q", info.CodeHome)
	}
}
