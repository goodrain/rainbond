package cnb

import (
	"os"
	"path"

	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultCNBBuilder is the default online CNB builder image
	DefaultCNBBuilder = "registry.cn-hangzhou.aliyuncs.com/goodrain/ubuntu-noble-builder:0.0.98"
	// DefaultCNBRunImage is the default online CNB run image
	DefaultCNBRunImage = "registry.cn-hangzhou.aliyuncs.com/goodrain/ubuntu-noble-run:0.0.73"
	// DefaultPHPCNBBuilder is the default Jammy Full builder image for PHP CNB builds.
	DefaultPHPCNBBuilder = "registry.cn-hangzhou.aliyuncs.com/goodrain/builder-jammy-full:0.3.613"
	// DefaultPHPCNBRunImage is the default Jammy Full run image for PHP CNB builds.
	DefaultPHPCNBRunImage = "registry.cn-hangzhou.aliyuncs.com/goodrain/run-jammy-full:0.1.141"
	// CNBLifecycleCreatorPath is the path to the lifecycle creator binary
	CNBLifecycleCreatorPath = "/lifecycle/creator"

	// Short image names for constructing internal registry references
	cnbBuilderShortName = "ubuntu-noble-builder:0.0.98"
	cnbRunShortName     = "ubuntu-noble-run:0.0.73"
	phpBuilderShortName = "builder-jammy-full:0.3.613"
	phpRunShortName     = "run-jammy-full:0.1.141"
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

func GetCNBBuilderImageForLanguage(lang code.Lang) string {
	if lang == code.PHP {
		if v := os.Getenv("CNB_BUILDER_IMAGE"); v != "" {
			return v
		}
		if isOfflineMode() {
			img := path.Join(builder.REGISTRYDOMAIN, phpBuilderShortName)
			logrus.Infof("Offline mode: using PHP CNB builder image from internal registry: %s", img)
			return img
		}
		return DefaultPHPCNBBuilder
	}
	return GetCNBBuilderImage()
}

func GetCNBRunImageForLanguage(lang code.Lang) string {
	if lang == code.PHP {
		if v := os.Getenv("CNB_RUN_IMAGE"); v != "" {
			return v
		}
		if isOfflineMode() {
			img := path.Join(builder.REGISTRYDOMAIN, phpRunShortName)
			logrus.Infof("Offline mode: using PHP CNB run image from internal registry: %s", img)
			return img
		}
		return DefaultPHPCNBRunImage
	}
	return GetCNBRunImage()
}
