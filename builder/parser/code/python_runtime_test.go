package code

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRuntimeByStrategyPythonDetectsPackageManagerAndProcfile(t *testing.T) {
	dir := t.TempDir()
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
	if _, ok := runtimeInfo["RUNTIMES"]; ok {
		t.Fatalf("expected python cnb runtime detection to ignore runtime.txt/version files, got %q", runtimeInfo["RUNTIMES"])
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

func TestCheckRuntimeByStrategyPythonDetectsFlaskStartCommand(t *testing.T) {
	dir := t.TempDir()
	pyproject := `[project]
name = "python-flask"
version = "0.1.0"
dependencies = [
    "Flask==3.1.3",
]
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		t.Fatalf("write pyproject.toml: %v", err)
	}
	appDir := filepath.Join(dir, "src", "python_flask")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.py"), []byte("from flask import Flask\n\n\ndef create_app():\n    return Flask(__name__)\n"), 0644); err != nil {
		t.Fatalf("write app.py: %v", err)
	}

	runtimeInfo, err := CheckRuntimeByStrategy(dir, Python, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy returned error: %v", err)
	}
	if got := runtimeInfo["PACKAGE_TOOL"]; got != "pip" {
		t.Fatalf("expected default package manager pip, got %q", got)
	}
	if got := runtimeInfo["START_CMD"]; got != "web: flask --app python_flask.app run --host 0.0.0.0 --port $PORT" {
		t.Fatalf("expected flask start command, got %q", got)
	}
	if got := runtimeInfo["START_CMD_SOURCE"]; got != "auto-detected" {
		t.Fatalf("expected auto-detected start command source, got %q", got)
	}
}

func TestCheckRuntimeByStrategyPythonDetectsUvicornPackageModuleStartCommand(t *testing.T) {
	dir := t.TempDir()
	pyproject := `[project]
name = "fastapi-demo"
version = "0.1.0"
dependencies = [
    "fastapi==0.115.0",
    "uvicorn==0.30.6",
]
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		t.Fatalf("write pyproject.toml: %v", err)
	}
	appDir := filepath.Join(dir, "src", "demo")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "main.py"), []byte("from fastapi import FastAPI\n\napp = FastAPI()\n"), 0644); err != nil {
		t.Fatalf("write main.py: %v", err)
	}

	runtimeInfo, err := CheckRuntimeByStrategy(dir, Python, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy returned error: %v", err)
	}
	if got := runtimeInfo["START_CMD"]; got != "web: uvicorn demo.main:app --host 0.0.0.0 --port $PORT" {
		t.Fatalf("expected uvicorn package module start command, got %q", got)
	}
}

func TestCheckRuntimeByStrategyPythonDetectsGunicornWsgiModuleStartCommand(t *testing.T) {
	dir := t.TempDir()
	requirements := "falcon==4.0.2\ngunicorn==23.0.0\n"
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(requirements), 0644); err != nil {
		t.Fatalf("write requirements.txt: %v", err)
	}
	appDir := filepath.Join(dir, "src", "demo")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "wsgi.py"), []byte("from falcon import App\n\napplication = App()\n"), 0644); err != nil {
		t.Fatalf("write wsgi.py: %v", err)
	}

	runtimeInfo, err := CheckRuntimeByStrategy(dir, Python, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy returned error: %v", err)
	}
	if got := runtimeInfo["START_CMD"]; got != "web: gunicorn demo.wsgi:application --bind 0.0.0.0:$PORT" {
		t.Fatalf("expected gunicorn wsgi start command, got %q", got)
	}
}

func TestCheckRuntimeByStrategyPythonDetectsQuartStartCommand(t *testing.T) {
	dir := t.TempDir()
	requirements := "quart==0.20.0\n"
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(requirements), 0644); err != nil {
		t.Fatalf("write requirements.txt: %v", err)
	}
	appDir := filepath.Join(dir, "src", "chat")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.py"), []byte("from quart import Quart\n\napp = Quart(__name__)\n"), 0644); err != nil {
		t.Fatalf("write app.py: %v", err)
	}

	runtimeInfo, err := CheckRuntimeByStrategy(dir, Python, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy returned error: %v", err)
	}
	if got := runtimeInfo["START_CMD"]; got != "web: hypercorn --bind 0.0.0.0:$PORT chat.app:app" {
		t.Fatalf("expected quart hypercorn start command, got %q", got)
	}
}

func TestCheckRuntimeByStrategyPythonDetectsSanicStartCommand(t *testing.T) {
	dir := t.TempDir()
	requirements := "sanic==24.6.0\n"
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(requirements), 0644); err != nil {
		t.Fatalf("write requirements.txt: %v", err)
	}
	appDir := filepath.Join(dir, "src", "demo")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "server.py"), []byte("from sanic import Sanic\n\napp = Sanic(\"demo\")\n"), 0644); err != nil {
		t.Fatalf("write server.py: %v", err)
	}

	runtimeInfo, err := CheckRuntimeByStrategy(dir, Python, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy returned error: %v", err)
	}
	if got := runtimeInfo["START_CMD"]; got != "web: sanic demo.server:app --host=0.0.0.0 --port=$PORT" {
		t.Fatalf("expected sanic start command, got %q", got)
	}
}

func TestCheckRuntimeByStrategyPythonDetectsAiohttpStartCommand(t *testing.T) {
	dir := t.TempDir()
	requirements := "aiohttp==3.10.5\n"
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(requirements), 0644); err != nil {
		t.Fatalf("write requirements.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.py"), []byte("from aiohttp import web\n\napp = web.Application()\n\nweb.run_app(app)\n"), 0644); err != nil {
		t.Fatalf("write server.py: %v", err)
	}

	runtimeInfo, err := CheckRuntimeByStrategy(dir, Python, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy returned error: %v", err)
	}
	if got := runtimeInfo["START_CMD"]; got != "web: python server.py" {
		t.Fatalf("expected aiohttp start command, got %q", got)
	}
}

func TestCheckRuntimeByStrategyPythonDetectsPyramidStartCommand(t *testing.T) {
	dir := t.TempDir()
	requirements := "pyramid==2.0.2\nwaitress==3.0.0\n"
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(requirements), 0644); err != nil {
		t.Fatalf("write requirements.txt: %v", err)
	}
	ini := `[app:main]
use = egg:demo

[server:main]
use = egg:waitress#main
listen = *:%(http_port)s
`
	if err := os.WriteFile(filepath.Join(dir, "development.ini"), []byte(ini), 0644); err != nil {
		t.Fatalf("write development.ini: %v", err)
	}

	runtimeInfo, err := CheckRuntimeByStrategy(dir, Python, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy returned error: %v", err)
	}
	if got := runtimeInfo["START_CMD"]; got != "web: pserve development.ini http_port=$PORT" {
		t.Fatalf("expected pyramid start command, got %q", got)
	}
}
