package sourceutil

import (
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/goodrain/rainbond/pkg/component/storage"
)

var localPackageSourcePrefixes = []string{"/grdata"}

// IsLocalPackageSource reports whether the source should be treated as a local path.
func IsLocalPackageSource(source string) bool {
	if source == "" {
		return false
	}
	if u, err := url.Parse(source); err == nil && u.Scheme != "" {
		return false
	}
	for _, prefix := range localPackageSourcePrefixes {
		if source == prefix || strings.HasPrefix(source, prefix+"/") {
			return true
		}
	}
	return false
}

// ReadLocalPackageDir ensures the local package directory is available on disk and returns its entries.
func ReadLocalPackageDir(sourcePath string) ([]os.FileInfo, error) {
	fileInfos, err := ioutil.ReadDir(sourcePath)
	if err == nil {
		return fileInfos, nil
	}

	if storage.Default() == nil || storage.Default().StorageCli == nil {
		return nil, err
	}
	if downloadErr := storage.Default().StorageCli.DownloadDirToDir(sourcePath, sourcePath); downloadErr != nil {
		return nil, downloadErr
	}

	fileInfos, err = ioutil.ReadDir(sourcePath)
	if err != nil {
		return nil, err
	}
	return fileInfos, nil
}
