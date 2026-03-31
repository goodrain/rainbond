package cnb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
)

func TestPythonLanguageConfigAnnotationsAndEnv(t *testing.T) {
	dir := t.TempDir()
	re := &build.Request{
		Lang:      code.Python,
		SourceDir: dir,
		BuildEnvs: map[string]string{
			"BUILD_RUNTIMES":           "3.11",
			"PIP_INDEX_URL":            "https://pypi.tuna.tsinghua.edu.cn/simple",
			"PIP_EXTRA_INDEX_URL":      "https://pypi.org/simple",
			"PIP_TRUSTED_HOST":         "pypi.tuna.tsinghua.edu.cn",
			"BP_CONDA_SOLVER":          "mamba",
			"BP_PIP_VERSION":           "23.3.1",
			"BUILD_PROCFILE":           "web: gunicorn app:app --bind 0.0.0.0:$PORT",
			"BUILD_POETRY_SOURCE_NAME": "private",
			"BUILD_POETRY_SOURCE_URL":  "https://poetry.example.com/simple",
			"BUILD_CONDA_CHANNEL_URL":  "https://conda.example.com/channel",
		},
	}

	if _, ok := getLanguageConfig(re).(*pythonConfig); !ok {
		t.Fatal("expected pythonConfig for python build")
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-cpython-version"] != "3.11" {
		t.Fatalf("expected cnb-bp-cpython-version=3.11, got %q", annotations["cnb-bp-cpython-version"])
	}
	if annotations["cnb-bp-conda-solver"] != "mamba" {
		t.Fatalf("expected cnb-bp-conda-solver=mamba, got %q", annotations["cnb-bp-conda-solver"])
	}
	if annotations["cnb-pip-index-url"] != "https://pypi.tuna.tsinghua.edu.cn/simple" {
		t.Fatalf("expected cnb-pip-index-url annotation, got %q", annotations["cnb-pip-index-url"])
	}
	if annotations["cnb-pip-extra-index-url"] != "https://pypi.org/simple" {
		t.Fatalf("expected cnb-pip-extra-index-url annotation, got %q", annotations["cnb-pip-extra-index-url"])
	}
	if annotations["cnb-pip-trusted-host"] != "pypi.tuna.tsinghua.edu.cn" {
		t.Fatalf("expected cnb-pip-trusted-host annotation, got %q", annotations["cnb-pip-trusted-host"])
	}
	if annotations["cnb-poetry-repositories-private-url"] != "https://poetry.example.com/simple" {
		t.Fatalf("expected poetry repository annotation, got %q", annotations["cnb-poetry-repositories-private-url"])
	}
	if annotations["cnb-conda-channels"] != "https://conda.example.com/channel" {
		t.Fatalf("expected conda channels annotation, got %q", annotations["cnb-conda-channels"])
	}
	if annotations["cnb-bp-pip-version"] != "23.3.1" {
		t.Fatalf("expected cnb-bp-pip-version=23.3.1, got %q", annotations["cnb-bp-pip-version"])
	}
	if annotations["rainbond.io/cnb-language"] != "python" {
		t.Fatalf("expected python debug annotation, got %q", annotations["rainbond.io/cnb-language"])
	}
	if annotations["rainbond.io/cnb-start-command-source"] != "procfile" {
		t.Fatalf("expected procfile source annotation, got %q", annotations["rainbond.io/cnb-start-command-source"])
	}

	if err := getLanguageConfig(re).InjectMirrorConfig(re); err != nil {
		t.Fatalf("InjectMirrorConfig returned error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "Procfile"))
	if err != nil {
		t.Fatalf("read Procfile: %v", err)
	}
	if string(content) != "web: gunicorn app:app --bind 0.0.0.0:$PORT\n" {
		t.Fatalf("unexpected Procfile content %q", string(content))
	}

	envs := (&Builder{}).buildEnvVars(re)
	found := map[string]string{}
	for _, env := range envs {
		found[env.Name] = env.Value
	}
	if found["PIP_INDEX_URL"] != "https://pypi.tuna.tsinghua.edu.cn/simple" {
		t.Fatalf("expected PIP_INDEX_URL env var, got %q", found["PIP_INDEX_URL"])
	}
	if found["PIP_EXTRA_INDEX_URL"] != "https://pypi.org/simple" {
		t.Fatalf("expected PIP_EXTRA_INDEX_URL env var, got %q", found["PIP_EXTRA_INDEX_URL"])
	}
	if found["PIP_TRUSTED_HOST"] != "pypi.tuna.tsinghua.edu.cn" {
		t.Fatalf("expected PIP_TRUSTED_HOST env var, got %q", found["PIP_TRUSTED_HOST"])
	}
}

func TestPythonLanguageConfigUsesAutoProcfileWhenUserOverrideMissing(t *testing.T) {
	dir := t.TempDir()
	re := &build.Request{
		Lang:      code.Python,
		SourceDir: dir,
		BuildEnvs: map[string]string{
			"BUILD_AUTO_PROCFILE":  "web: uvicorn main:app --host 0.0.0.0 --port $PORT",
			"START_COMMAND_SOURCE": "auto-detected",
		},
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["rainbond.io/cnb-start-command-source"] != "auto-detected" {
		t.Fatalf("expected auto-detected start command source, got %q", annotations["rainbond.io/cnb-start-command-source"])
	}
	if annotations["rainbond.io/cnb-start-command-hint"] != "web: uvicorn main:app --host 0.0.0.0 --port $PORT" {
		t.Fatalf("expected start command hint from BUILD_AUTO_PROCFILE, got %q", annotations["rainbond.io/cnb-start-command-hint"])
	}

	if err := getLanguageConfig(re).InjectMirrorConfig(re); err != nil {
		t.Fatalf("InjectMirrorConfig returned error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "Procfile"))
	if err != nil {
		t.Fatalf("read Procfile: %v", err)
	}
	if string(content) != "web: uvicorn main:app --host 0.0.0.0 --port $PORT\n" {
		t.Fatalf("unexpected Procfile content %q", string(content))
	}
}
