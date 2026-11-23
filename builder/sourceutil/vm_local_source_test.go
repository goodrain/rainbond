package sourceutil

import (
	"os"
	"path/filepath"
	"testing"
)

// capability_id: rainbond.vm-build.local-source-boundary
func TestVMLocalSourceOnlyAllowsManagedSingleEventDirectory(t *testing.T) {
	for _, source := range []string{"/grdata/package_build/temp/events/owned-test", "/grdata/package_build/temp/events/abc_123-456", "/grdata/package_build/temp/events/_owned", "/grdata/package_build/temp/events/-owned"} {
		if err := ValidateVMLocalPackageSource(source); err != nil {
			t.Fatal("valid managed source rejected", err)
		}
	}
	for _, source := range []string{"/grdata", "/grdata/other", "/grdata/package_build/temp/events", "/grdata/package_build/temp/events/../victim", "/grdata/package_build/temp/events/id/../victim", "/grdata/package_build/temp/events/id/child", "/grdata/package_build/temp/events//id", "/grdata/package_build/temp/events/id/", "/grdata/package_build/temp/events/.hidden", "/grdata/package_build/temp/events/id\n", "/grdata/package_build/temp/events/中文", "/tmp/events/id"} {
		if err := ValidateVMLocalPackageSource(source); err == nil {
			t.Fatal("out of scope local source accepted", source)
		}
	}
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "keep")
	if err := os.WriteFile(sentinel, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RemoveVMLocalPackageSource(outside); err == nil {
		t.Fatal("cleanup accepted outside directory")
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatal("outside directory was modified", err)
	}
}

// capability_id: rainbond.vm-build.local-source-boundary
func TestVMLocalSourceRejectsAnyDirectorySymlinkOrNonDirectory(t *testing.T) {
	dir := t.TempDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "file")
	if err = os.WriteFile(file, []byte("do not read or delete"), 0600); err != nil {
		t.Fatal(err)
	}
	fileInfo, _ := os.Stat(file)
	link := filepath.Join(dir, "link")
	if err = os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	linkInfo, _ := os.Lstat(link)
	source := "/grdata/package_build/temp/events/owned"
	for _, bad := range []string{"/grdata", "/grdata/package_build", "/grdata/package_build/temp", "/grdata/package_build/temp/events", source} {
		for _, badInfo := range []os.FileInfo{fileInfo, linkInfo} {
			err := validateVMLocalPackageSource(source, func(name string) (os.FileInfo, error) {
				if name == bad {
					return badInfo, nil
				}
				return info, nil
			})
			if err == nil {
				t.Fatal("unsafe local path component accepted", bad)
			}
		}
	}
}
