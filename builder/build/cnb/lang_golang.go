package cnb

import (
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	corev1 "k8s.io/api/core/v1"
)

type golangConfig struct{}

func (g *golangConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	if version := firstNonEmptyEnv(re.BuildEnvs, "BP_GO_VERSION", "BUILD_GOVERSION", "GOVERSION"); version != "" {
		annotations["cnb-bp-go-version"] = version
	}
	if targets := strings.TrimSpace(re.BuildEnvs["BUILD_GO_INSTALL_PACKAGE_SPEC"]); targets != "" {
		annotations["cnb-bp-go-targets"] = targets
	}
}

func (g *golangConfig) BuildEnvVars(re *build.Request) []corev1.EnvVar {
	var envs []corev1.EnvVar
	if value := strings.TrimSpace(re.BuildEnvs["BUILD_GOPROXY"]); value != "" {
		envs = append(envs, corev1.EnvVar{Name: "GOPROXY", Value: value})
	}
	if value := strings.TrimSpace(re.BuildEnvs["BUILD_GOPRIVATE"]); value != "" {
		envs = append(envs, corev1.EnvVar{Name: "GOPRIVATE", Value: value})
	}
	return envs
}

func (g *golangConfig) InjectMirrorConfig(re *build.Request) error {
	return ensureProcfile(re)
}

func (g *golangConfig) CustomOrder(re *build.Request) []orderBuildpack {
	return nil
}
