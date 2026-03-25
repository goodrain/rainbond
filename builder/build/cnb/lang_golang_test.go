package cnb

import (
	"testing"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
)

func TestGolangLanguageConfigAnnotationsAndEnv(t *testing.T) {
	re := &build.Request{
		Lang:      code.Golang,
		SourceDir: t.TempDir(),
		BuildEnvs: map[string]string{
			"BUILD_GOVERSION":               "1.23",
			"BUILD_GOPROXY":                 "https://goproxy.cn",
			"BUILD_GOPRIVATE":               "github.com/example/*",
			"BUILD_GO_INSTALL_PACKAGE_SPEC": "./cmd/api",
		},
	}

	if _, ok := getLanguageConfig(re).(*golangConfig); !ok {
		t.Fatal("expected golangConfig for golang build")
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-go-version"] != "1.23" {
		t.Fatalf("expected cnb-bp-go-version=1.23, got %q", annotations["cnb-bp-go-version"])
	}
	if annotations["cnb-bp-go-targets"] != "./cmd/api" {
		t.Fatalf("expected cnb-bp-go-targets=./cmd/api, got %q", annotations["cnb-bp-go-targets"])
	}
	if annotations["rainbond.io/cnb-language"] != "golang" {
		t.Fatalf("expected golang debug annotation, got %q", annotations["rainbond.io/cnb-language"])
	}

	envs := (&Builder{}).buildEnvVars(re)
	foundProxy := false
	foundPrivate := false
	for _, env := range envs {
		if env.Name == "GOPROXY" && env.Value == "https://goproxy.cn" {
			foundProxy = true
		}
		if env.Name == "GOPRIVATE" && env.Value == "github.com/example/*" {
			foundPrivate = true
		}
	}
	if !foundProxy || !foundPrivate {
		t.Fatal("expected GOPROXY and GOPRIVATE env vars for golang cnb build")
	}
}
