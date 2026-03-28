package cnb

import (
	"os"
	"path/filepath"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
	corev1 "k8s.io/api/core/v1"
)

// LanguageConfig provides language-specific CNB build configuration.
// Each supported language implements this interface to customize
// annotations, mirror injection, and buildpack ordering.
type LanguageConfig interface {
	// BuildAnnotations adds language-specific BP_* annotations.
	BuildAnnotations(re *build.Request, annotations map[string]string)
	// BuildEnvVars adds language-specific environment variables to the build container.
	BuildEnvVars(re *build.Request) []corev1.EnvVar
	// InjectMirrorConfig injects language-specific mirror/proxy config files.
	InjectMirrorConfig(re *build.Request) error
	// CustomOrder returns custom buildpack order, or nil to use builder default.
	CustomOrder(re *build.Request) []orderBuildpack
}

// getLanguageConfig returns the LanguageConfig for the build request.
// Dispatches based on project type: pure static (no package.json) or Node.js.
// Future languages (Python, Go, etc.) will be added here.
func getLanguageConfig(re *build.Request) LanguageConfig {
	switch re.Lang {
	case code.JavaMaven, code.JaveWar, code.JavaJar, code.Gradle:
		return &javaConfig{}
	case code.Python:
		return &pythonConfig{}
	case code.Golang:
		return &golangConfig{}
	case code.PHP:
		return &phpConfig{}
	case code.NetCore:
		return &dotnetConfig{}
	case code.Static:
		return &staticConfig{}
	case code.Nodejs:
		if isPureStaticProject(re.SourceDir) {
			return &staticConfig{}
		}
		return &nodejsConfig{}
	default:
		if isPureStaticProject(re.SourceDir) {
			return &staticConfig{}
		}
		return &nodejsConfig{}
	}
}

func ensureProcfile(re *build.Request) error {
	procfile := firstNonEmptyEnv(re.BuildEnvs, "BUILD_PROCFILE", "BUILD_AUTO_PROCFILE")
	if procfile == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(re.SourceDir, "Procfile"), []byte(procfile+"\n"), 0644)
}
