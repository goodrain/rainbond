package cnb

import (
	"os"
	"path"

	"github.com/goodrain/rainbond/builder"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultCNBBuilder is the default online CNB builder image
	DefaultCNBBuilder = "registry.cn-hangzhou.aliyuncs.com/goodrain/ubuntu-noble-builder:0.0.72"
	// DefaultCNBRunImage is the default online CNB run image
	DefaultCNBRunImage = "registry.cn-hangzhou.aliyuncs.com/goodrain/ubuntu-noble-run:0.0.50"
	// CNBLifecycleCreatorPath is the path to the lifecycle creator binary
	CNBLifecycleCreatorPath = "/lifecycle/creator"

	// Short image names for constructing internal registry references
	cnbBuilderShortName = "ubuntu-noble-builder:0.0.72"
	cnbRunShortName     = "ubuntu-noble-run:0.0.50"
)

// isOfflineMode checks whether the cluster is in offline/air-gapped mode
// by looking for the same marker file used by getDependencyMirror.
func isOfflineMode() bool {
	_, err := os.Stat(offlineMirrorMarker)
	return err == nil
}

// GetCNBBuilderImage returns the CNB builder image reference.
// Priority: env var > offline (REGISTRYDOMAIN) > default online URL.
func GetCNBBuilderImage() string {
	if v := os.Getenv("CNB_BUILDER_IMAGE"); v != "" {
		return v
	}
	if isOfflineMode() {
		img := path.Join(builder.REGISTRYDOMAIN, cnbBuilderShortName)
		logrus.Infof("Offline mode: using CNB builder image from internal registry: %s", img)
		return img
	}
	return DefaultCNBBuilder
}

// GetCNBRunImage returns the CNB run image reference.
// Priority: env var > offline (REGISTRYDOMAIN) > default online URL.
func GetCNBRunImage() string {
	if v := os.Getenv("CNB_RUN_IMAGE"); v != "" {
		return v
	}
	if isOfflineMode() {
		img := path.Join(builder.REGISTRYDOMAIN, cnbRunShortName)
		logrus.Infof("Offline mode: using CNB run image from internal registry: %s", img)
		return img
	}
	return DefaultCNBRunImage
}
