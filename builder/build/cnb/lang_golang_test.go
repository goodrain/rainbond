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
			"BP_GO_VERSION":           "1.25",
			"GOPROXY":                 "https://goproxy.cn",
			"GOPRIVATE":               "github.com/example/*",
			"BP_GO_TARGETS":           "./cmd/api",
			"BP_GO_BUILD_FLAGS":       "-trimpath",
			"BP_GO_BUILD_LDFLAGS":     "-s -w",
			"BP_GO_BUILD_IMPORT_PATH": "example.com/custom",
			"BP_KEEP_FILES":           "static/**",
			"BP_GO_WORK_USE":          "auto",
			"BP_LIVE_RELOAD_ENABLED":  "true",
		},
	}

	if _, ok := getLanguageConfig(re).(*golangConfig); !ok {
		t.Fatal("expected golangConfig for golang build")
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-go-version"] != "1.25" {
		t.Fatalf("expected cnb-bp-go-version=1.25, got %q", annotations["cnb-bp-go-version"])
	}
	if annotations["cnb-bp-go-targets"] != "./cmd/api" {
		t.Fatalf("expected cnb-bp-go-targets=./cmd/api, got %q", annotations["cnb-bp-go-targets"])
	}
	if annotations["cnb-bp-go-build-flags"] != "-trimpath" {
		t.Fatalf("expected cnb-bp-go-build-flags, got %q", annotations["cnb-bp-go-build-flags"])
	}
	if annotations["cnb-bp-go-build-ldflags"] != "-s -w" {
		t.Fatalf("expected cnb-bp-go-build-ldflags, got %q", annotations["cnb-bp-go-build-ldflags"])
	}
	if annotations["cnb-bp-go-build-import-path"] != "example.com/custom" {
		t.Fatalf("expected cnb-bp-go-build-import-path, got %q", annotations["cnb-bp-go-build-import-path"])
	}
	if annotations["cnb-bp-keep-files"] != "static/**" {
		t.Fatalf("expected cnb-bp-keep-files, got %q", annotations["cnb-bp-keep-files"])
	}
	if annotations["cnb-bp-go-work-use"] != "auto" {
		t.Fatalf("expected cnb-bp-go-work-use, got %q", annotations["cnb-bp-go-work-use"])
	}
	if annotations["cnb-bp-live-reload-enabled"] != "true" {
		t.Fatalf("expected cnb-bp-live-reload-enabled, got %q", annotations["cnb-bp-live-reload-enabled"])
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
