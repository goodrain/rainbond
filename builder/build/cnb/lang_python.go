package cnb

import (
	"github.com/goodrain/rainbond/builder/build"
	corev1 "k8s.io/api/core/v1"
)

type pythonConfig struct{}

func (p *pythonConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	applyDependencyMirrorAnnotation(annotations)
	setAnnotationValue(annotations, "cnb-bp-cpython-version", firstNonEmptyEnv(re.BuildEnvs, "BP_CPYTHON_VERSION", "BUILD_RUNTIMES", "RUNTIMES"))
	setAnnotationValue(annotations, "cnb-bp-conda-solver", re.BuildEnvs["BUILD_CONDA_SOLVER"])
	setAnnotationValue(annotations, "cnb-bp-live-reload-enabled", re.BuildEnvs["BUILD_LIVE_RELOAD_ENABLED"])
}

func (p *pythonConfig) BuildEnvVars(re *build.Request) []corev1.EnvVar {
	var envs []corev1.EnvVar
	envs = appendEnvVar(envs, "PIP_INDEX_URL", re.BuildEnvs["BUILD_PIP_INDEX_URL"])
	return envs
}

func (p *pythonConfig) InjectMirrorConfig(re *build.Request) error {
	return ensureProcfile(re)
}

func (p *pythonConfig) CustomOrder(re *build.Request) []orderBuildpack {
	return nil
}
