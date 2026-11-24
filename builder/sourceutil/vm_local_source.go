package sourceutil

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

const vmUploadEventRoot = "/grdata/package_build/temp/events"

var vmUploadEventID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func invalidVMLocalSource() error {
	return fmt.Errorf("vm_local_source_invalid: 本地虚拟机镜像只允许使用受管上传事件目录")
}

// ValidateVMLocalPackageSource applies only to VM uploads, not other application
// source kinds. Missing directories may be materialized by the storage client;
// every existing ancestor must be a real directory, never a symlink.
func ValidateVMLocalPackageSource(source string) error {
	return validateVMLocalPackageSource(source, os.Lstat)
}

func validateVMLocalPackageSource(source string, lstat func(string) (os.FileInfo, error)) error {
	if !strings.HasPrefix(source, vmUploadEventRoot+"/") {
		return invalidVMLocalSource()
	}
	id := strings.TrimPrefix(source, vmUploadEventRoot+"/")
	if !vmUploadEventID.MatchString(id) {
		return invalidVMLocalSource()
	}
	current := ""
	for _, part := range strings.Split(strings.TrimPrefix(source, "/"), "/") {
		current += "/" + part
		info, err := lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil || info == nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return invalidVMLocalSource()
		}
	}
	return nil
}

// RemoveVMLocalPackageSource repeats the boundary/ancestor check immediately
// before cleanup, rather than trusting a validation from before the build.
func RemoveVMLocalPackageSource(source string) error {
	if err := ValidateVMLocalPackageSource(source); err != nil {
		return err
	}
	return os.RemoveAll(source)
}
