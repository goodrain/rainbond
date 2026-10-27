package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
)

// PreparePackageBuildWorkspace extracts the selected uploaded archive into a
// per-build directory. The caller must release it after the build has finished.
// Standalone JAR/WAR inputs retain their existing handling.
func PreparePackageBuildWorkspace(sourceDir, archiveName string) (string, func(), error) {
	if strings.TrimSpace(sourceDir) == "" {
		return "", nil, fmt.Errorf("invalid package source directory")
	}
	if strings.TrimSpace(archiveName) == "" || strings.ContainsAny(archiveName, "/\\") || filepath.Base(archiveName) != archiveName {
		return "", nil, fmt.Errorf("invalid package archive name")
	}
	archivePath := filepath.Join(sourceDir, archiveName)
	info, err := os.Lstat(archivePath)
	if err != nil {
		return "", nil, fmt.Errorf("read uploaded package: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("uploaded package is not a regular file")
	}
	ext, archived := packageArchiveExt(archiveName)
	if !archived {
		switch strings.ToLower(filepath.Ext(archiveName)) {
		case ".jar", ".war":
			return sourceDir, func() {}, nil
		default:
			return "", nil, fmt.Errorf("unsupported source package format")
		}
	}
	workspace, err := os.MkdirTemp(filepath.Dir(sourceDir), ".cnb-package-")
	if err != nil {
		return "", nil, fmt.Errorf("create package build workspace: %w", err)
	}
	cleanup := func() {
		if err := os.RemoveAll(workspace); err != nil {
			logrus.Warnf("remove package build workspace failed: %v", err)
		}
	}
	if ext == ".zip" {
		err = util.Unzip(archivePath, workspace)
	} else {
		err = util.UnTar(archivePath, workspace, ext != ".tar")
	}
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("extract uploaded package: %w", err)
	}
	return workspace, cleanup, nil
}
