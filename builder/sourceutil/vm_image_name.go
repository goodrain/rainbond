package sourceutil

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var vmImageFileNamePattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}._-]*$`)
var vmVMDKInputNamePattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{M}\p{N} ._-]*$`)

// IsSafeVMImageFileName excludes paths and Dockerfile instruction syntax.
func IsSafeVMImageFileName(name string) bool {
	return vmImageFileNamePattern.MatchString(name)
}

// VMDK is staged under a fixed safe name before Dockerfile rendering. This
// input-only policy accommodates VMware's default basename without admitting
// paths, shell syntax, control/bidi characters or option-looking names.
func IsSafeVMVMDKInputFileName(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".vmdk") && vmVMDKInputNamePattern.MatchString(name)
}

func IsWrappedVMVMDKFileName(name string) bool {
	name = strings.ToLower(name)
	for _, suffix := range []string{".vmdk.gz", ".vmdk.xz", ".vmdk.tar", ".vmdk.tar.gz", ".vmdk.tar.xz"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// VMImageFileNameFromURL decodes only the final escaped path segment once.
// Query strings, fragments and encoded separators cannot become local paths.
func VMImageFileNameFromURL(source string) (string, error) {
	u, err := url.Parse(source)
	if err != nil || u.Host == "" || (!strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) {
		return "", fmt.Errorf("invalid VM image URL")
	}
	escapedPath := u.EscapedPath()
	if escapedPath == "" || strings.HasSuffix(escapedPath, "/") {
		return "", fmt.Errorf("VM image URL requires a file name")
	}
	name, err := url.PathUnescape(path.Base(escapedPath))
	if err != nil || !IsSafeVMImageFileName(name) && !IsSafeVMVMDKInputFileName(name) {
		return "", fmt.Errorf("unsafe VM image URL file name")
	}
	return name, nil
}
