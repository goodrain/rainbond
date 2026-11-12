package sourceutil

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var vmImageFileNamePattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}._-]*$`)

// IsSafeVMImageFileName excludes paths and Dockerfile instruction syntax.
func IsSafeVMImageFileName(name string) bool {
	return vmImageFileNamePattern.MatchString(name)
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
	if err != nil || !IsSafeVMImageFileName(name) {
		return "", fmt.Errorf("unsafe VM image URL file name")
	}
	return name, nil
}
