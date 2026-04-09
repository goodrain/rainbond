package compose

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goodrain/rainbond/util"
)

const defaultComposeCacheRoot = "/cache/docker-compose"

func createComposeCacheDir() (string, func(), error) {
	cacheRoot := os.Getenv("RAINBOND_COMPOSE_CACHE_DIR")
	if cacheRoot == "" {
		cacheRoot = defaultComposeCacheRoot
	}

	if err := util.CheckAndCreateDir(cacheRoot); err == nil {
		cacheDir, err := os.MkdirTemp(cacheRoot, "compose-")
		if err == nil {
			return cacheDir, func() { _ = os.RemoveAll(cacheDir) }, nil
		}
	}

	cacheDir, err := os.MkdirTemp("", "rainbond-docker-compose-")
	if err != nil {
		return "", nil, err
	}
	return cacheDir, func() { _ = os.RemoveAll(cacheDir) }, nil
}

func writeComposeBodies(cacheDir string, bodys [][]byte) ([]string, error) {
	files := make([]string, 0, len(bodys))
	for i, body := range bodys {
		filename := filepath.Join(cacheDir, stringFileName(i))
		if err := os.WriteFile(filename, body, 0755); err != nil {
			return nil, err
		}
		files = append(files, filename)
	}
	return files, nil
}

func stringFileName(index int) string {
	return fmt.Sprintf("%d-docker-compose.yml", index)
}
