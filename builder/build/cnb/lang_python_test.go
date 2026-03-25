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
			"BUILD_RUNTIMES":      "3.11",
			"BUILD_PIP_INDEX_URL": "https://pypi.tuna.tsinghua.edu.cn/simple",
			"BUILD_PROCFILE":      "web: gunicorn app:app --bind 0.0.0.0:$PORT",
		},
	}

	if _, ok := getLanguageConfig(re).(*pythonConfig); !ok {
		t.Fatal("expected pythonConfig for python build")
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-cpython-version"] != "3.11" {
		t.Fatalf("expected cnb-bp-cpython-version=3.11, got %q", annotations["cnb-bp-cpython-version"])
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
	found := false
	for _, env := range envs {
		if env.Name == "PIP_INDEX_URL" && env.Value == "https://pypi.tuna.tsinghua.edu.cn/simple" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected PIP_INDEX_URL env var for python cnb build")
	}
}
