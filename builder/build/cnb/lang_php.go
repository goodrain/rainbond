package cnb

import (
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	corev1 "k8s.io/api/core/v1"
)

type phpConfig struct{}

func (p *phpConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	applyDependencyMirrorAnnotation(annotations)
	if version := firstNonEmptyEnv(re.BuildEnvs, "BP_PHP_VERSION", "BUILD_RUNTIMES", "RUNTIMES"); version != "" {
		setAnnotationValue(annotations, "cnb-bp-php-version", version)
	}
	server := strings.TrimSpace(re.BuildEnvs["BUILD_RUNTIMES_SERVER"])
	if server == "" {
		server = "nginx"
	}
	setAnnotationValue(annotations, "cnb-bp-php-server", server)

	setAnnotationValue(annotations, "cnb-bp-composer-version", firstNonEmptyEnv(re.BuildEnvs, "BP_COMPOSER_VERSION", "BUILD_COMPOSER_VERSION"))
	setAnnotationValue(annotations, "cnb-bp-composer-install-options", firstNonEmptyEnv(re.BuildEnvs, "BP_COMPOSER_INSTALL_OPTIONS", "BUILD_COMPOSER_INSTALL_OPTIONS"))
	setAnnotationValue(annotations, "cnb-bp-composer-install-global", re.BuildEnvs["BUILD_COMPOSER_INSTALL_GLOBAL"])
	setAnnotationValue(annotations, "cnb-bp-php-web-dir", firstNonEmptyEnv(re.BuildEnvs, "BP_PHP_WEB_DIR", "BUILD_PHP_WEB_DIR"))
	if truthyBuildEnv(re.BuildEnvs["BUILD_PHP_NGINX_ENABLE_HTTPS"]) {
		setAnnotationValue(annotations, "cnb-bp-php-nginx-enable-https", "true")
	}
	if truthyBuildEnv(re.BuildEnvs["BUILD_PHP_ENABLE_HTTPS_REDIRECT"]) {
		setAnnotationValue(annotations, "cnb-bp-php-enable-https-redirect", "true")
	}
}

func (p *phpConfig) BuildEnvVars(re *build.Request) []corev1.EnvVar {
	var envs []corev1.EnvVar
	envs = appendEnvVar(envs, "COMPOSER_VENDOR_DIR", firstNonEmptyEnv(re.BuildEnvs, "COMPOSER_VENDOR_DIR", "BUILD_COMPOSER_VENDOR_DIR"))
	envs = appendEnvVar(envs, "COMPOSER", firstNonEmptyEnv(re.BuildEnvs, "COMPOSER", "BUILD_COMPOSER_FILE"))
	envs = appendEnvVar(envs, "COMPOSER_AUTH", firstNonEmptyEnv(re.BuildEnvs, "COMPOSER_AUTH", "BUILD_COMPOSER_AUTH"))
	return envs
}

func (p *phpConfig) InjectMirrorConfig(re *build.Request) error {
	return ensureProcfile(re)
}

func (p *phpConfig) CustomOrder(re *build.Request) []orderBuildpack {
	return nil
}
