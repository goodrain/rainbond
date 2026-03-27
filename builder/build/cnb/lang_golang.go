package cnb

import (
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	corev1 "k8s.io/api/core/v1"
)

type golangConfig struct{}

func (g *golangConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	applyDependencyMirrorAnnotation(annotations)
	if version := firstNonEmptyEnv(re.BuildEnvs, "BP_GO_VERSION", "BUILD_GOVERSION", "GOVERSION"); version != "" {
		setAnnotationValue(annotations, "cnb-bp-go-version", version)
	}
	if targets := strings.TrimSpace(re.BuildEnvs["BUILD_GO_INSTALL_PACKAGE_SPEC"]); targets != "" {
		setAnnotationValue(annotations, "cnb-bp-go-targets", targets)
	}
	setAnnotationValue(annotations, "cnb-bp-go-build-flags", re.BuildEnvs["BUILD_GO_BUILD_FLAGS"])
	setAnnotationValue(annotations, "cnb-bp-go-build-ldflags", re.BuildEnvs["BUILD_GO_BUILD_LDFLAGS"])
	setAnnotationValue(annotations, "cnb-bp-go-build-import-path", re.BuildEnvs["BUILD_GO_BUILD_IMPORT_PATH"])
	setAnnotationValue(annotations, "cnb-bp-keep-files", re.BuildEnvs["BUILD_GO_KEEP_FILES"])
	setAnnotationValue(annotations, "cnb-bp-go-work-use", re.BuildEnvs["BUILD_GO_WORK_USE"])
	if truthyBuildEnv(re.BuildEnvs["BUILD_LIVE_RELOAD_ENABLED"]) {
		setAnnotationValue(annotations, "cnb-bp-live-reload-enabled", "true")
	}
}

func (g *golangConfig) BuildEnvVars(re *build.Request) []corev1.EnvVar {
	var envs []corev1.EnvVar
	envs = appendEnvVar(envs, "GOPROXY", re.BuildEnvs["BUILD_GOPROXY"])
	envs = appendEnvVar(envs, "GOPRIVATE", re.BuildEnvs["BUILD_GOPRIVATE"])
	return envs
}

func (g *golangConfig) InjectMirrorConfig(re *build.Request) error {
	return ensureProcfile(re)
}

func (g *golangConfig) CustomOrder(re *build.Request) []orderBuildpack {
	return nil
}
