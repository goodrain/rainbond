package cnb

import (
	"fmt"
	"os"
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
	corev1 "k8s.io/api/core/v1"
)

const (
	defaultCNBMavenVersion     = "3.9.14"
	pythonPackageManagerPip    = "pip"
	pythonPackageManagerPipenv = "pipenv"
	pythonPackageManagerPoetry = "poetry"
	pythonPackageManagerConda  = "conda"
)

type platformBinding struct {
	Name          string
	Type          string
	ConfigMapName string
	ConfigMapKey  string
	TargetFile    string
}

func ensureBuildEnvs(re *build.Request) {
	if re.BuildEnvs == nil {
		re.BuildEnvs = map[string]string{}
	}
}

func truthyBuildEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}

func appendEnvVar(envs []corev1.EnvVar, name, value string) []corev1.EnvVar {
	value = strings.TrimSpace(value)
	if value == "" {
		return envs
	}
	return append(envs, corev1.EnvVar{Name: name, Value: value})
}

func setAnnotationValue(annotations map[string]string, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	annotations[key] = value
}

func validateSupportedBuildParams(re *build.Request) error {
	ensureBuildEnvs(re)
	if strings.TrimSpace(re.BuildStrategy) != "cnb" {
		return nil
	}

	switch re.Lang {
	case code.JavaMaven, code.Gradle, code.JaveWar:
		re.BuildEnvs["BUILD_RUNTIMES_MAVEN"] = defaultCNBMavenVersion
		server := firstNonEmptyEnv(re.BuildEnvs, "BP_JAVA_APP_SERVER", "BUILD_RUNTIMES_SERVER")
		if server == "" && re.Lang == code.JaveWar {
			server = "tomcat"
		}
		if normalized, err := normalizeJavaAppServer(server); err != nil {
			return err
		} else if normalized != "" {
			re.BuildEnvs["BP_JAVA_APP_SERVER"] = normalized
			re.BuildEnvs["BUILD_RUNTIMES_SERVER"] = normalized
		}
	case code.Python:
		manager, err := detectPythonPackageManager(re)
		if err != nil {
			return err
		}
		if manager != "" {
			re.BuildEnvs["BUILD_PYTHON_PACKAGE_MANAGER"] = manager
		}
		if manager == pythonPackageManagerConda {
			solver := firstNonEmptyEnv(re.BuildEnvs, "BP_CONDA_SOLVER", "BUILD_CONDA_SOLVER")
			if solver == "" {
				solver = "mamba"
			}
			re.BuildEnvs["BP_CONDA_SOLVER"] = solver
			re.BuildEnvs["BUILD_CONDA_SOLVER"] = solver
		}
		if version := strings.TrimSpace(re.BuildEnvs["BUILD_PYTHON_PACKAGE_MANAGER_VERSION"]); version != "" {
			envName, err := pythonPackageManagerVersionEnv(manager)
			if err != nil {
				return err
			}
			re.BuildEnvs[envName] = version
		}
	case code.PHP:
		if normalized, err := normalizePHPServer(re.BuildEnvs["BUILD_RUNTIMES_SERVER"]); err != nil {
			return err
		} else if normalized != "" {
			re.BuildEnvs["BUILD_RUNTIMES_SERVER"] = normalized
		}
	}

	return nil
}

func normalizeJavaAppServer(server string) (string, error) {
	server = strings.ToLower(strings.TrimSpace(server))
	if server == "" {
		return "", nil
	}
	switch {
	case strings.Contains(server, "tomcat"):
		return "tomcat", nil
	case strings.Contains(server, "tomee"):
		return "tomee", nil
	case strings.Contains(server, "liberty"):
		return "liberty", nil
	default:
		return "", fmt.Errorf("unsupported java app server %q", server)
	}
}

func normalizePHPServer(server string) (string, error) {
	server = strings.ToLower(strings.TrimSpace(server))
	if server == "" {
		return "", nil
	}
	switch server {
	case "apache", "httpd":
		return "httpd", nil
	case "nginx", "php-server":
		return server, nil
	default:
		return "", fmt.Errorf("unsupported php server %q", server)
	}
}

func detectPythonPackageManager(re *build.Request) (string, error) {
	manager, err := normalizePythonPackageManager(re.BuildEnvs["BUILD_PYTHON_PACKAGE_MANAGER"])
	if err != nil {
		return "", err
	}
	if manager != "" {
		return manager, nil
	}
	return code.DetectPythonPackageManager(re.SourceDir), nil
}

func normalizePythonPackageManager(manager string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(manager)) {
	case "":
		return "", nil
	case pythonPackageManagerPip, pythonPackageManagerPipenv, pythonPackageManagerPoetry, pythonPackageManagerConda:
		return strings.ToLower(strings.TrimSpace(manager)), nil
	default:
		return "", fmt.Errorf("unsupported python package manager %q", manager)
	}
}

func pythonPackageManagerVersionEnv(manager string) (string, error) {
	switch manager {
	case pythonPackageManagerPip:
		return "BP_PIP_VERSION", nil
	case pythonPackageManagerPipenv:
		return "BP_PIPENV_VERSION", nil
	case pythonPackageManagerPoetry:
		return "BP_POETRY_VERSION", nil
	case pythonPackageManagerConda:
		return "", fmt.Errorf("conda package manager does not support BUILD_PYTHON_PACKAGE_MANAGER_VERSION")
	default:
		return "", fmt.Errorf("python package manager must be set before BUILD_PYTHON_PACKAGE_MANAGER_VERSION can be used")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func bindingTypeAnnotationKey(bindingName string) string {
	bindingName = strings.ToLower(strings.TrimSpace(bindingName))
	if bindingName == "" {
		return "cnb-binding-type"
	}
	var normalized strings.Builder
	for _, r := range bindingName {
		switch {
		case r >= 'a' && r <= 'z':
			normalized.WriteRune(r)
		case r >= '0' && r <= '9':
			normalized.WriteRune(r)
		case r == '-':
			normalized.WriteRune(r)
		default:
			normalized.WriteRune('-')
		}
	}
	return "cnb-binding-" + normalized.String() + "-type"
}
