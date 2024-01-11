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
	"strings"
	"testing"
	"time"
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

func TestGetCurrentDir(t *testing.T) {
	t.Log(GetCurrentDir())
}

func TestCopyFile(t *testing.T) {
	if err := CopyFile("/tmp/test2.zip", "/tmp/test4.zip"); err != nil {
		t.Fatal(err)
	}
}

func TestParseVariable(t *testing.T) {
	configs := make(map[string]string, 0)
	result := ParseVariable("sada${XXX:aaa}dasd${XXX:aaa} ${YYY:aaa} ASDASD ${ZZZ:aaa}", configs)
	t.Log(result)

	t.Log(ParseVariable("sada${XXX:aaa}dasd${XXX:aaa} ${YYY:aaa} ASDASD ${ZZZ:aaa}", map[string]string{
		"XXX": "123DDD",
		"ZZZ": ",.,.,.,.",
	}))
}

func TestTimeFormat(t *testing.T) {
	tt := "2019-08-24 11:11:30.165753932 +0800 CST m=+55557.682499470"
	timeF, err := time.Parse(time.RFC3339, strings.Replace(tt[0:19]+"+08:00", " ", "T", 1))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(timeF.Format(time.RFC3339))
}
