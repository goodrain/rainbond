package cnb

import (
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
	corev1 "k8s.io/api/core/v1"
)

type javaConfig struct{}

func (j *javaConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	if version := firstNonEmptyEnv(re.BuildEnvs, "BP_JVM_VERSION", "BUILD_RUNTIMES", "RUNTIMES"); version != "" {
		annotations["cnb-bp-jvm-version"] = version
	}

	goals := strings.TrimSpace(re.BuildEnvs["BUILD_MAVEN_CUSTOM_GOALS"])
	opts := strings.TrimSpace(re.BuildEnvs["BUILD_MAVEN_CUSTOM_OPTS"])
	buildArgs := strings.TrimSpace(strings.Join([]string{goals, opts}, " "))
	if buildArgs != "" {
		annotations["cnb-bp-maven-build-arguments"] = strings.Join(strings.Fields(buildArgs), " ")
	}

	server := strings.TrimSpace(re.BuildEnvs["BUILD_RUNTIMES_SERVER"])
	if server == "" && re.Lang == code.JaveWar {
		server = "tomcat"
	}
	if server != "" {
		annotations["cnb-bp-java-app-server"] = server
	}
}

func (j *javaConfig) BuildEnvVars(re *build.Request) []corev1.EnvVar {
	if value := strings.TrimSpace(re.BuildEnvs["BUILD_MAVEN_JAVA_OPTS"]); value != "" {
		return []corev1.EnvVar{{Name: "MAVEN_OPTS", Value: value}}
	}
	return nil
}

func (j *javaConfig) InjectMirrorConfig(re *build.Request) error {
	return ensureProcfile(re)
}

func (j *javaConfig) CustomOrder(re *build.Request) []orderBuildpack {
	return nil
}
