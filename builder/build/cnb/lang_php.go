package cnb

import (
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	corev1 "k8s.io/api/core/v1"
)

type phpConfig struct{}

func (p *phpConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	if version := firstNonEmptyEnv(re.BuildEnvs, "BP_PHP_VERSION", "BUILD_RUNTIMES", "RUNTIMES"); version != "" {
		annotations["cnb-bp-php-version"] = version
	}
	server := strings.TrimSpace(re.BuildEnvs["BUILD_RUNTIMES_SERVER"])
	if server == "" {
		server = "nginx"
	}
	annotations["cnb-bp-php-server"] = server
}

func (p *phpConfig) BuildEnvVars(re *build.Request) []corev1.EnvVar {
	return nil
}

func (p *phpConfig) InjectMirrorConfig(re *build.Request) error {
	return ensureProcfile(re)
}

func (p *phpConfig) CustomOrder(re *build.Request) []orderBuildpack {
	return nil
}
