package cnb

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// defaultOnlineMirror is the public object storage URL for CNB dependencies.
const defaultOnlineMirror = "https://buildpack.rainbond.com/cnb"

// offlineMirrorMarker is the file path inside the build pod (grdata mount)
// that an offline provisioning tool writes to switch to local file:// mirror.
var offlineMirrorMarker = "/grdata/cnb/BP_DEPENDENCY_MIRROR"

func applyDependencyMirrorAnnotation(annotations map[string]string) {
	setAnnotationValue(annotations, "cnb-bp-dependency-mirror", getDependencyMirror())
}

// getDependencyMirror returns the CNB dependency mirror URL.
// Priority: env var > offline marker file > default online URL.
//
// For offline/air-gapped environments, write the mirror URL into the marker
// file at /opt/rainbond/grdata/cnb/BP_DEPENDENCY_MIRROR. Example content:
//
//	file://../../../../grdata/cnb
//
// Note: Paketo resolves file:// URIs relative to the buildpack install dir
// (/cnb/buildpacks/xxx/version/), so use ../../../../ to traverse back to /.
func getDependencyMirror() string {
	if v := strings.TrimSpace(os.Getenv("BP_DEPENDENCY_MIRROR")); v != "" {
		return v
	}
	if data, err := os.ReadFile(offlineMirrorMarker); err == nil {
		if m := strings.TrimSpace(string(data)); m != "" {
			logrus.Infof("Using dependency mirror from %s: %s", offlineMirrorMarker, m)
			return m
		}
	}
	return defaultOnlineMirror
}
