package cnb

import (
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	corev1 "k8s.io/api/core/v1"
)

type pythonConfig struct{}

func (p *pythonConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	if version := firstNonEmptyEnv(re.BuildEnvs, "BP_CPYTHON_VERSION", "BUILD_RUNTIMES", "RUNTIMES"); version != "" {
		annotations["cnb-bp-cpython-version"] = version
	}
}

func (p *pythonConfig) BuildEnvVars(re *build.Request) []corev1.EnvVar {
	if value := strings.TrimSpace(re.BuildEnvs["BUILD_PIP_INDEX_URL"]); value != "" {
		return []corev1.EnvVar{{Name: "PIP_INDEX_URL", Value: value}}
	}
	return nil
}

func (p *pythonConfig) InjectMirrorConfig(re *build.Request) error {
	return ensureProcfile(re)
}

func (p *pythonConfig) CustomOrder(re *build.Request) []orderBuildpack {
	return nil
}
