package code

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRuntimeByStrategyPythonDetectsPackageManagerAndProcfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "runtime.txt"), []byte("python-3.11.9"), 0644); err != nil {
		t.Fatalf("write runtime.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Pipfile"), []byte("[packages]\nflask='*'\n"), 0644); err != nil {
		t.Fatalf("write Pipfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Procfile"), []byte("web: gunicorn app:app --bind 0.0.0.0:$PORT\n"), 0644); err != nil {
		t.Fatalf("write Procfile: %v", err)
	}

	runtimeInfo, err := CheckRuntimeByStrategy(dir, Python, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy returned error: %v", err)
	}
	if got := runtimeInfo["RUNTIMES"]; got != "3.11" {
		t.Fatalf("expected normalized python runtime 3.11, got %q", got)
	}
	if got := runtimeInfo["PACKAGE_TOOL"]; got != "pipenv" {
		t.Fatalf("expected package manager pipenv, got %q", got)
	}
	if got := runtimeInfo["START_CMD"]; got != "web: gunicorn app:app --bind 0.0.0.0:$PORT" {
		t.Fatalf("expected Procfile start command, got %q", got)
	}
	if got := runtimeInfo["START_CMD_SOURCE"]; got != "procfile" {
		t.Fatalf("expected start command source procfile, got %q", got)
	}
}

func TestCheckRuntimeByStrategyPythonDetectsDjangoStartCommand(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "runtime.txt"), []byte("python-3.12.1"), 0644); err != nil {
		t.Fatalf("write runtime.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manage.py"), []byte("print('django')"), 0644); err != nil {
		t.Fatalf("write manage.py: %v", err)
	}
	projectDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir demo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "settings.py"), []byte("SECRET_KEY='x'"), 0644); err != nil {
		t.Fatalf("write settings.py: %v", err)
	}

	runtimeInfo, err := CheckRuntimeByStrategy(dir, Python, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy returned error: %v", err)
	}
	if got := runtimeInfo["PACKAGE_TOOL"]; got != "pip" {
		t.Fatalf("expected default package manager pip, got %q", got)
	}
	if got := runtimeInfo["START_CMD"]; got != "web: python manage.py runserver 0.0.0.0:$PORT" {
		t.Fatalf("expected django start command, got %q", got)
	}
	if got := runtimeInfo["START_CMD_SOURCE"]; got != "auto-detected" {
		t.Fatalf("expected auto-detected start command source, got %q", got)
	}
}

func TestCheckRuntimeByStrategyPythonDetectsPoetryScriptStartCommand(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "runtime.txt"), []byte("python-3.12.2"), 0644); err != nil {
		t.Fatalf("write runtime.txt: %v", err)
	}
	pyproject := `[tool.poetry]
name = "demo"
version = "0.1.0"

[tool.poetry.scripts]
web = "demo:main"
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		t.Fatalf("write pyproject.toml: %v", err)
	}

	runtimeInfo, err := CheckRuntimeByStrategy(dir, Python, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy returned error: %v", err)
	}
	if got := runtimeInfo["PACKAGE_TOOL"]; got != "poetry" {
		t.Fatalf("expected package manager poetry, got %q", got)
	}
	if got := runtimeInfo["START_CMD"]; got != "web: poetry run web" {
		t.Fatalf("expected poetry run start command, got %q", got)
	}
	if got := runtimeInfo["START_CMD_SOURCE"]; got != "auto-detected" {
		t.Fatalf("expected auto-detected start command source, got %q", got)
	}
}
