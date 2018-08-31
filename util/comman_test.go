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

package util

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenOrCreateFile(t *testing.T) {
	file, err := OpenOrCreateFile("./test.log")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
}

func TestDeweight(t *testing.T) {
	data := []string{"asd", "asd", "12", "12"}
	Deweight(&data)
	t.Log(data)
}

func TestGetDirSize(t *testing.T) {
	t.Log(GetDirSize("/go"))
	memStats := &runtime.MemStats{}
	runtime.ReadMemStats(memStats)
}

func TestGetDirSizeByCmd(t *testing.T) {
	t.Log(GetDirSizeByCmd("/go"))
	memStats := &runtime.MemStats{}
	runtime.ReadMemStats(memStats)
}

func TestZip(t *testing.T) {
	if err := Zip("/tmp/cache", "/tmp/cache.zip"); err != nil {
		t.Fatal(err)
	}
}

func TestUnzip(t *testing.T) {
	if err := Unzip("/tmp/cache.zip", "/tmp/cache0"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateVersionByTime(t *testing.T) {
	if re := CreateVersionByTime(); re != "" {
		t.Log(re)
	}
}

func TestGetDirList(t *testing.T) {
	list, err := GetDirList("/tmp", 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(list)
}

func TestMergeDir(t *testing.T) {
	t.Log(filepath.Dir("/tmp/cache/asdasd"))
	if err := MergeDir("/tmp/ctr-944254844/", "/tmp/cache"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateHostID(t *testing.T) {
	uid, err := CreateHostID()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(uid)
}

func TestDiskUsage(t *testing.T) {
	total, free := DiskUsage("/Users/qingguo")
	t.Logf("%d GB,%d MB", total/1024/1024/1024, free/1024/1024)
}

func TestGetCurrentDir(t *testing.T) {
	t.Log(GetCurrentDir())
}
