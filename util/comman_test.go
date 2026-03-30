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
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/goodrain/rainbond/util/zip"
)

// capability_id: rainbond.util.fs.archive-and-directory-ops
func TestOpenOrCreateFile(t *testing.T) {
	file, err := OpenOrCreateFile(filepath.Join(t.TempDir(), "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
}

// capability_id: rainbond.util.system.identity-and-template-helpers
// capability_id: rainbond.util.array-deduplicate
func TestDeweight(t *testing.T) {
	data := []string{"asd", "asd", "12", "12"}
	Deweight(&data)
	if len(data) != 2 || data[0] != "asd" || data[1] != "12" {
		t.Fatalf("unexpected deweight result: %#v", data)
	}
}

// capability_id: rainbond.util.fs.archive-and-directory-ops
// capability_id: rainbond.util.dir-size-walk
func TestGetDirSize(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte(strings.Repeat("a", 1024)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte(strings.Repeat("b", 1024)), 0644); err != nil {
		t.Fatal(err)
	}
	if got := GetDirSize(root); got != 2 {
		t.Fatalf("GetDirSize(%q)=%v, want 2", root, got)
	}
	memStats := &runtime.MemStats{}
	runtime.ReadMemStats(memStats)
}

// capability_id: rainbond.util.fs.archive-and-directory-ops
// capability_id: rainbond.util.dir-size-shell
func TestGetDirSizeByCmd(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte(strings.Repeat("a", 2048)), 0644); err != nil {
		t.Fatal(err)
	}
	if got := GetDirSizeByCmd(root); got <= 0 {
		t.Fatalf("expected positive dir size, got %v", got)
	}
	memStats := &runtime.MemStats{}
	runtime.ReadMemStats(memStats)
}

// capability_id: rainbond.util.fs.archive-and-directory-ops
// capability_id: rainbond.util.zip-archive
func TestZip(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "cache")
	target := filepath.Join(root, "cache.zip")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "app.txt"), []byte("rainbond"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Zip(source, target); err != nil {
		t.Fatal(err)
	}
	if ok, err := FileExists(target); err != nil || !ok {
		t.Fatalf("expected zip file %s to exist, err=%v", target, err)
	}
}

// capability_id: rainbond.util.system.identity-and-template-helpers
// capability_id: rainbond.util.version-timestamp
func TestCreateVersionByTime(t *testing.T) {
	if re := CreateVersionByTime(); re != "" {
		if len(re) != 14 {
			t.Fatalf("expected 14-digit version, got %q", re)
		}
		for _, r := range re {
			if !unicode.IsDigit(r) {
				t.Fatalf("expected digits only, got %q", re)
			}
		}
	} else {
		t.Fatal("expected version")
	}
}

// capability_id: rainbond.util.fs.archive-and-directory-ops
// capability_id: rainbond.util.dir-list-depth
func TestGetDirList(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "x", "y"), 0755); err != nil {
		t.Fatal(err)
	}
	list, err := GetDirList(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(list)
	want := []string{filepath.Join(root, "a", "b"), filepath.Join(root, "x", "y")}
	if len(list) != len(want) || list[0] != want[0] || list[1] != want[1] {
		t.Fatalf("GetDirList=%v, want %v", list, want)
	}
}

// capability_id: rainbond.util.file-list-depth
func TestGetFileList(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "x.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "y.txt"), []byte("y"), 0644); err != nil {
		t.Fatal(err)
	}
	list, err := GetFileList(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(list)
	if len(list) != 2 {
		t.Fatalf("unexpected file list: %v", list)
	}
}

// capability_id: rainbond.util.dir-name-list
func TestGetDirNameList(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "b"), 0755); err != nil {
		t.Fatal(err)
	}
	list, err := GetDirNameList(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(list)
	if len(list) != 2 || list[0] != "a" || list[1] != "b" {
		t.Fatalf("unexpected dir names: %v", list)
	}
}

// capability_id: rainbond.util.zip-structure-detect
func TestDetectZipStructure(t *testing.T) {
	files := []*zip.File{
		{FileHeader: zip.FileHeader{Name: "demo/file1.txt"}},
		{FileHeader: zip.FileHeader{Name: "demo/sub/file2.txt"}},
	}
	ok, root := detectZipStructure(files)
	if !ok || root != "demo" {
		t.Fatalf("expected common root demo, got ok=%v root=%q", ok, root)
	}

	files = []*zip.File{
		{FileHeader: zip.FileHeader{Name: "demo/file1.txt"}},
		{FileHeader: zip.FileHeader{Name: "other/file2.txt"}},
	}
	ok, root = detectZipStructure(files)
	if ok || root != "demo" {
		t.Fatalf("expected no common root, got ok=%v root=%q", ok, root)
	}
}

// capability_id: rainbond.util.fs.archive-and-directory-ops
func TestMergeDir(t *testing.T) {
	root := t.TempDir()
	fromDir := filepath.Join(root, "from")
	toDir := filepath.Join(root, "to")
	if err := os.MkdirAll(fromDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(toDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fromDir, "hello.txt"), []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := MergeDir(fromDir, toDir); err != nil {
		t.Fatal(err)
	}
	if ok, err := FileExists(filepath.Join(toDir, "hello.txt")); err != nil || !ok {
		t.Fatalf("expected merged file, err=%v", err)
	}
}

// capability_id: rainbond.util.system.identity-and-template-helpers
// capability_id: rainbond.util.host-id-generate
func TestCreateHostID(t *testing.T) {
	uid, err := CreateHostID()
	if err != nil {
		t.Fatal(err)
	}
	if len(uid) != 32 {
		t.Fatalf("expected 32-char host id, got %q", uid)
	}
}

// capability_id: rainbond.util.system.identity-and-template-helpers
// capability_id: rainbond.util.current-dir-path
func TestGetCurrentDir(t *testing.T) {
	if dir := GetCurrentDir(); dir == "" || !filepath.IsAbs(dir) {
		t.Fatalf("expected absolute current dir, got %q", dir)
	}
}

// capability_id: rainbond.util.fs.archive-and-directory-ops
// capability_id: rainbond.util.file-copy
func TestCopyFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(source, []byte("copy-me"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := CopyFile(source, target); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "copy-me" {
		t.Fatalf("unexpected copied content: %q", string(content))
	}
}

// capability_id: rainbond.util.system.identity-and-template-helpers
// capability_id: rainbond.util.template-variable-parse
func TestParseVariable(t *testing.T) {
	configs := make(map[string]string, 0)
	result := ParseVariable("sada${XXX:aaa}dasd${XXX:aaa} ${YYY:aaa} ASDASD ${ZZZ:aaa}", configs)
	if result != "sadaaaadasdaaa aaa ASDASD aaa" {
		t.Fatalf("unexpected default parse result: %q", result)
	}

	got := ParseVariable("sada${XXX:aaa}dasd${XXX:aaa} ${YYY:aaa} ASDASD ${ZZZ:aaa}", map[string]string{
		"XXX": "123DDD",
		"ZZZ": ",.,.,.,.",
	})
	if got != "sada123DDDdasd123DDD aaa ASDASD ,.,.,.,." {
		t.Fatalf("unexpected configured parse result: %q", got)
	}
}

// capability_id: rainbond.util.system.identity-and-template-helpers
// capability_id: rainbond.util.time-format-rfc3339
func TestTimeFormat(t *testing.T) {
	tt := "2019-08-24 11:11:30.165753932 +0800 CST m=+55557.682499470"
	timeF, err := time.Parse(time.RFC3339, strings.Replace(tt[0:19]+"+08:00", " ", "T", 1))
	if err != nil {
		t.Fatal(err)
	}
	if got := timeF.Format(time.RFC3339); got != "2019-08-24T11:11:30+08:00" {
		t.Fatalf("unexpected formatted time: %q", got)
	}
}

// capability_id: rainbond.util.etcd-key-id-parse
func TestGetIDFromKey(t *testing.T) {
	if got := GetIDFromKey("/services/demo-abc123"); got != "demo" {
		t.Fatalf("unexpected id from hyphenated key: %q", got)
	}
	if got := GetIDFromKey("/services/demo"); got != "demo" {
		t.Fatalf("unexpected id from plain key: %q", got)
	}
	if got := GetIDFromKey("invalid"); got != "" {
		t.Fatalf("expected empty id for invalid key, got %q", got)
	}
}

// capability_id: rainbond.util.whitespace-filter
func TestRemoveSpaces(t *testing.T) {
	got := RemoveSpaces([]string{"a", " ", "\t", "", "b", "\n", "c"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("unexpected result length: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("RemoveSpaces()=%v, want %v", got, want)
		}
	}
}

// capability_id: rainbond.util.getenv
func TestGetenv(t *testing.T) {
	t.Setenv("RBD_TEST_ENV_GETENV", "")
	if got := Getenv("RBD_TEST_ENV_GETENV", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback env, got %q", got)
	}

	t.Setenv("RBD_TEST_ENV_GETENV", "value")
	if got := Getenv("RBD_TEST_ENV_GETENV", "fallback"); got != "value" {
		t.Fatalf("expected explicit env value, got %q", got)
	}
}

// capability_id: rainbond.util.env-default
func TestGetenvDefault(t *testing.T) {
	t.Setenv("RBD_TEST_ENV", "")
	if got := GetenvDefault("RBD_TEST_ENV", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback env, got %q", got)
	}

	t.Setenv("RBD_TEST_ENV", "value")
	if got := GetenvDefault("RBD_TEST_ENV", "fallback"); got != "value" {
		t.Fatalf("expected explicit env value, got %q", got)
	}
}

// capability_id: rainbond.util.statefulset-suffix-detect
func TestIsEndWithNumber(t *testing.T) {
	ok, suffix := IsEndWithNumber("mysql-0")
	if !ok || suffix != "-0" {
		t.Fatalf("unexpected result for mysql-0: ok=%v suffix=%q", ok, suffix)
	}

	ok, suffix = IsEndWithNumber("mysql")
	if ok || suffix != "" {
		t.Fatalf("expected no numeric suffix, got ok=%v suffix=%q", ok, suffix)
	}
}
