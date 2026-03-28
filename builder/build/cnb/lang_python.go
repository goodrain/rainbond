package cnb

import (
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	corev1 "k8s.io/api/core/v1"
)

type pythonConfig struct{}

func (p *pythonConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	applyDependencyMirrorAnnotation(annotations)
	setAnnotationValue(annotations, "cnb-bp-cpython-version", firstNonEmptyEnv(re.BuildEnvs, "BP_CPYTHON_VERSION", "BUILD_RUNTIMES", "RUNTIMES"))
	setAnnotationValue(annotations, "cnb-bp-conda-solver", firstNonEmptyEnv(re.BuildEnvs, "BP_CONDA_SOLVER", "BUILD_CONDA_SOLVER"))
	setAnnotationValue(annotations, "cnb-bp-live-reload-enabled", re.BuildEnvs["BUILD_LIVE_RELOAD_ENABLED"])

	setAnnotationValue(annotations, bpEnvToAnnotationKey("PIP_INDEX_URL"), firstNonEmptyEnv(re.BuildEnvs, "PIP_INDEX_URL", "BUILD_PIP_INDEX_URL"))
	setAnnotationValue(annotations, bpEnvToAnnotationKey("PIP_EXTRA_INDEX_URL"), firstNonEmptyEnv(re.BuildEnvs, "PIP_EXTRA_INDEX_URL", "BUILD_PIP_EXTRA_INDEX_URL"))
	setAnnotationValue(annotations, bpEnvToAnnotationKey("PIP_TRUSTED_HOST"), firstNonEmptyEnv(re.BuildEnvs, "PIP_TRUSTED_HOST", "BUILD_PIP_TRUSTED_HOST"))

	poetrySourceName := strings.TrimSpace(re.BuildEnvs["BUILD_POETRY_SOURCE_NAME"])
	poetrySourceURL := strings.TrimSpace(re.BuildEnvs["BUILD_POETRY_SOURCE_URL"])
	if poetrySourceName != "" && poetrySourceURL != "" {
		envName := fmt.Sprintf("POETRY_REPOSITORIES_%s_URL", normalizeEnvNameToken(poetrySourceName))
		setAnnotationValue(annotations, bpEnvToAnnotationKey(envName), poetrySourceURL)
	}

	condaChannelURL := firstNonEmptyEnv(re.BuildEnvs, "CONDA_CHANNELS", "BUILD_CONDA_CHANNEL_URL")
	setAnnotationValue(annotations, bpEnvToAnnotationKey("CONDA_CHANNELS"), condaChannelURL)
}

func (p *pythonConfig) BuildEnvVars(re *build.Request) []corev1.EnvVar {
	var envs []corev1.EnvVar
	envs = appendEnvVar(envs, "PIP_INDEX_URL", firstNonEmptyEnv(re.BuildEnvs, "PIP_INDEX_URL", "BUILD_PIP_INDEX_URL"))
	envs = appendEnvVar(envs, "PIP_EXTRA_INDEX_URL", firstNonEmptyEnv(re.BuildEnvs, "PIP_EXTRA_INDEX_URL", "BUILD_PIP_EXTRA_INDEX_URL"))
	envs = appendEnvVar(envs, "PIP_TRUSTED_HOST", firstNonEmptyEnv(re.BuildEnvs, "PIP_TRUSTED_HOST", "BUILD_PIP_TRUSTED_HOST"))

	poetrySourceName := strings.TrimSpace(re.BuildEnvs["BUILD_POETRY_SOURCE_NAME"])
	poetrySourceURL := strings.TrimSpace(re.BuildEnvs["BUILD_POETRY_SOURCE_URL"])
	if poetrySourceName != "" && poetrySourceURL != "" {
		envName := fmt.Sprintf("POETRY_REPOSITORIES_%s_URL", normalizeEnvNameToken(poetrySourceName))
		envs = appendEnvVar(envs, envName, poetrySourceURL)
	}
	envs = appendEnvVar(envs, "CONDA_CHANNELS", firstNonEmptyEnv(re.BuildEnvs, "CONDA_CHANNELS", "BUILD_CONDA_CHANNEL_URL"))
	return envs
}

func (p *pythonConfig) InjectMirrorConfig(re *build.Request) error {
	return ensureProcfile(re)
}

func (p *pythonConfig) CustomOrder(re *build.Request) []orderBuildpack {
	return nil
}

func normalizeEnvNameToken(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	return strings.Trim(builder.String(), "_")
}
