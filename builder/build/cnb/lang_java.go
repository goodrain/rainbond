package cnb

import (
	"path/filepath"
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
	multi "github.com/goodrain/rainbond/builder/parser/code/multisvc"
	corev1 "k8s.io/api/core/v1"
)

type javaConfig struct{}

func (j *javaConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	applyDependencyMirrorAnnotation(annotations)
	setAnnotationValue(annotations, "cnb-bp-jvm-version", firstNonEmptyEnv(re.BuildEnvs, "BP_JVM_VERSION", "BUILD_RUNTIMES", "RUNTIMES"))

	goals := strings.TrimSpace(re.BuildEnvs["BUILD_MAVEN_CUSTOM_GOALS"])
	opts := strings.TrimSpace(re.BuildEnvs["BUILD_MAVEN_CUSTOM_OPTS"])
	setAnnotationValue(annotations, "cnb-bp-maven-build-arguments", goals)
	setAnnotationValue(annotations, "cnb-bp-maven-additional-build-arguments", opts)
	setAnnotationValue(annotations, "cnb-bp-maven-version", re.BuildEnvs["BUILD_RUNTIMES_MAVEN"])
	builtModule, builtArtifact := resolveMavenBuiltTarget(re)
	setAnnotationValue(annotations, "cnb-bp-maven-built-module", builtModule)
	setAnnotationValue(annotations, "cnb-bp-maven-built-artifact", builtArtifact)

	if re.Lang == code.Gradle {
		setAnnotationValue(annotations, "cnb-bp-gradle-build-arguments", re.BuildEnvs["BUILD_GRADLE_BUILD_ARGUMENTS"])
		setAnnotationValue(annotations, "cnb-bp-gradle-additional-build-arguments", re.BuildEnvs["BUILD_GRADLE_ADDITIONAL_BUILD_ARGUMENTS"])
		setAnnotationValue(annotations, "cnb-bp-gradle-built-module", re.BuildEnvs["BUILD_GRADLE_BUILT_MODULE"])
		setAnnotationValue(annotations, "cnb-bp-gradle-built-artifact", re.BuildEnvs["BUILD_GRADLE_BUILT_ARTIFACT"])
	}

	server := strings.TrimSpace(re.BuildEnvs["BUILD_RUNTIMES_SERVER"])
	if server == "" && re.Lang == code.JaveWar {
		server = "tomcat"
	}
	setAnnotationValue(annotations, "cnb-bp-java-app-server", server)
}

func (j *javaConfig) BuildEnvVars(re *build.Request) []corev1.EnvVar {
	var envs []corev1.EnvVar
	builtModule, builtArtifact := resolveMavenBuiltTarget(re)
	envs = appendEnvVar(envs, "BP_MAVEN_BUILD_ARGUMENTS", re.BuildEnvs["BUILD_MAVEN_CUSTOM_GOALS"])
	envs = appendEnvVar(envs, "BP_MAVEN_ADDITIONAL_BUILD_ARGUMENTS", re.BuildEnvs["BUILD_MAVEN_CUSTOM_OPTS"])
	envs = appendEnvVar(envs, "BP_MAVEN_BUILT_MODULE", builtModule)
	envs = appendEnvVar(envs, "BP_MAVEN_BUILT_ARTIFACT", builtArtifact)
	envs = appendEnvVar(envs, "MAVEN_OPTS", re.BuildEnvs["BUILD_MAVEN_JAVA_OPTS"])
	return envs
}

func (j *javaConfig) InjectMirrorConfig(re *build.Request) error {
	return ensureProcfile(re)
}

func (j *javaConfig) CustomOrder(re *build.Request) []orderBuildpack {
	return nil
}

func resolveMavenBuiltTarget(re *build.Request) (string, string) {
	explicitModule := strings.TrimSpace(re.BuildEnvs["BUILD_MAVEN_BUILT_MODULE"])
	explicitArtifact := strings.TrimSpace(re.BuildEnvs["BUILD_MAVEN_BUILT_ARTIFACT"])
	if explicitModule != "" || explicitArtifact != "" {
		return explicitModule, explicitArtifact
	}
	if re.Lang != code.JavaMaven {
		return "", ""
	}
	if !fileExists(filepath.Join(re.SourceDir, "pom.xml")) {
		return "", ""
	}

	services, err := multi.NewMaven().ListModules(re.SourceDir)
	if err != nil || len(services) != 1 {
		return "", ""
	}
	moduleName := strings.TrimSpace(services[0].Name)
	if moduleName == "" || filepath.IsAbs(moduleName) {
		return "", ""
	}

	builtArtifact := ""
	if services[0].Envs != nil && services[0].Envs["BUILD_MAVEN_BUILT_ARTIFACT"] != nil {
		builtArtifact = services[0].Envs["BUILD_MAVEN_BUILT_ARTIFACT"].Value
	}
	return moduleName, strings.TrimSpace(builtArtifact)
}
