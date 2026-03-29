package cnb

import (
	"github.com/goodrain/rainbond/builder/build"
	corev1 "k8s.io/api/core/v1"
)

type dotnetConfig struct{}

func (d *dotnetConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	applyDependencyMirrorAnnotation(annotations)
	setAnnotationValue(annotations, "cnb-bp-dotnet-framework-version", firstNonEmptyEnv(re.BuildEnvs, "BP_DOTNET_FRAMEWORK_VERSION"))
	setAnnotationValue(annotations, "cnb-bp-dotnet-project-path", firstNonEmptyEnv(re.BuildEnvs, "BP_DOTNET_PROJECT_PATH"))
	setAnnotationValue(annotations, "cnb-bp-dotnet-publish-flags", firstNonEmptyEnv(re.BuildEnvs, "BP_DOTNET_PUBLISH_FLAGS"))
}

func (d *dotnetConfig) BuildEnvVars(re *build.Request) []corev1.EnvVar {
	return nil
}

func (d *dotnetConfig) InjectMirrorConfig(re *build.Request) error {
	return ensureProcfile(re)
}

func (d *dotnetConfig) CustomOrder(re *build.Request) []orderBuildpack {
	return nil
}
