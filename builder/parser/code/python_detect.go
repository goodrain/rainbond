package code

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	PythonStartSourceProcfile     = "procfile"
	PythonStartSourceAutoDetected = "auto-detected"
)

func DetectPythonPackageManager(buildPath string) string {
	for _, filename := range []string{"environment.yml", "environment.yaml", "conda.yml", "conda.yaml"} {
		if fileExists(filepath.Join(buildPath, filename)) {
			return "conda"
		}
	}
	if fileExists(filepath.Join(buildPath, "Pipfile")) {
		return "pipenv"
	}
	pyprojectPath := filepath.Join(buildPath, "pyproject.toml")
	if fileExists(pyprojectPath) {
		body, err := os.ReadFile(pyprojectPath)
		if err == nil && strings.Contains(string(body), "[tool.poetry]") {
			return "poetry"
		}
	}
	return "pip"
}

func DetectPythonStartCommand(buildPath, packageManager string) (string, string) {
	if ok, procfile := CheckProcfile(buildPath, Python); ok {
		return strings.TrimSpace(procfile), PythonStartSourceProcfile
	}

	if packageManager == "poetry" {
		if script := detectPoetryScriptName(buildPath); script != "" {
			return fmt.Sprintf("web: poetry run %s", script), PythonStartSourceAutoDetected
		}
	}

	if fileExists(filepath.Join(buildPath, "manage.py")) {
		return "web: python manage.py runserver 0.0.0.0:$PORT", PythonStartSourceAutoDetected
	}

	if pythonManifestContainsDependency(buildPath, "uvicorn") {
		if fileExists(filepath.Join(buildPath, "main.py")) {
			return "web: uvicorn main:app --host 0.0.0.0 --port $PORT", PythonStartSourceAutoDetected
		}
		if fileExists(filepath.Join(buildPath, "app.py")) {
			return "web: uvicorn app:app --host 0.0.0.0 --port $PORT", PythonStartSourceAutoDetected
		}
	}

	if pythonManifestContainsDependency(buildPath, "gunicorn") {
		if fileExists(filepath.Join(buildPath, "app.py")) {
			return "web: gunicorn app:app --bind 0.0.0.0:$PORT", PythonStartSourceAutoDetected
		}
		if fileExists(filepath.Join(buildPath, "main.py")) {
			return "web: gunicorn main:app --bind 0.0.0.0:$PORT", PythonStartSourceAutoDetected
		}
	}

	return "", ""
}

func detectPoetryScriptName(buildPath string) string {
	pyprojectPath := filepath.Join(buildPath, "pyproject.toml")
	body, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return ""
	}
	content := string(body)
	blockIndex := strings.Index(content, "[tool.poetry.scripts]")
	if blockIndex == -1 {
		return ""
	}
	block := content[blockIndex:]
	lines := strings.Split(block, "\n")
	pattern := regexp.MustCompile(`^\s*([A-Za-z0-9._-]+)\s*=`)
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			break
		}
		matches := pattern.FindStringSubmatch(trimmed)
		if len(matches) == 2 {
			return matches[1]
		}
	}
	return ""
}

func pythonManifestContainsDependency(buildPath string, dependency string) bool {
	dependency = strings.ToLower(strings.TrimSpace(dependency))
	for _, filename := range []string{"requirements.txt", "Pipfile", "pyproject.toml"} {
		body, err := os.ReadFile(filepath.Join(buildPath, filename))
		if err == nil && strings.Contains(strings.ToLower(string(body)), dependency) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
