package cnb

import (
	"github.com/goodrain/rainbond/builder/build"
	corev1 "k8s.io/api/core/v1"
)

type golangConfig struct{}

func (g *golangConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	applyDependencyMirrorAnnotation(annotations)
	if version := firstNonEmptyEnv(re.BuildEnvs, "BP_GO_VERSION", "BUILD_GOVERSION", "GOVERSION"); version != "" {
		setAnnotationValue(annotations, "cnb-bp-go-version", version)
	}
	setAnnotationValue(annotations, "cnb-goproxy", firstNonEmptyEnv(re.BuildEnvs, "GOPROXY", "BUILD_GOPROXY"))
	setAnnotationValue(annotations, "cnb-goprivate", firstNonEmptyEnv(re.BuildEnvs, "GOPRIVATE", "BUILD_GOPRIVATE"))
	if targets := firstNonEmptyEnv(re.BuildEnvs, "BP_GO_TARGETS", "BUILD_GO_INSTALL_PACKAGE_SPEC"); targets != "" {
		setAnnotationValue(annotations, "cnb-bp-go-targets", targets)
	}
	setAnnotationValue(annotations, "cnb-bp-go-build-flags", firstNonEmptyEnv(re.BuildEnvs, "BP_GO_BUILD_FLAGS", "BUILD_GO_BUILD_FLAGS"))
	setAnnotationValue(annotations, "cnb-bp-go-build-ldflags", firstNonEmptyEnv(re.BuildEnvs, "BP_GO_BUILD_LDFLAGS", "BUILD_GO_BUILD_LDFLAGS"))
	setAnnotationValue(annotations, "cnb-bp-go-build-import-path", firstNonEmptyEnv(re.BuildEnvs, "BP_GO_BUILD_IMPORT_PATH", "BUILD_GO_BUILD_IMPORT_PATH"))
	setAnnotationValue(annotations, "cnb-bp-keep-files", firstNonEmptyEnv(re.BuildEnvs, "BP_KEEP_FILES", "BUILD_GO_KEEP_FILES"))
	setAnnotationValue(annotations, "cnb-bp-go-work-use", firstNonEmptyEnv(re.BuildEnvs, "BP_GO_WORK_USE", "BUILD_GO_WORK_USE"))
	if truthyBuildEnv(firstNonEmptyEnv(re.BuildEnvs, "BP_LIVE_RELOAD_ENABLED", "BUILD_LIVE_RELOAD_ENABLED")) {
		setAnnotationValue(annotations, "cnb-bp-live-reload-enabled", "true")
	}
}

func (g *golangConfig) BuildEnvVars(re *build.Request) []corev1.EnvVar {
	return nil
}

func (g *golangConfig) InjectMirrorConfig(re *build.Request) error {
	return ensureProcfile(re)
}

func (g *golangConfig) CustomOrder(re *build.Request) []orderBuildpack {
	return nil
}
