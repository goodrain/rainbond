package cnb

import (
	"testing"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
)

func TestDotnetLanguageConfigSelection(t *testing.T) {
	re := &build.Request{
		Lang:      code.NetCore,
		SourceDir: t.TempDir(),
		BuildEnvs: map[string]string{
			"BP_DOTNET_FRAMEWORK_VERSION": "8.0",
			"BP_DOTNET_PROJECT_PATH":      "./src/WebApp",
			"BP_DOTNET_PUBLISH_FLAGS":     "--verbosity=normal",
		},
	}

	if _, ok := getLanguageConfig(re).(*dotnetConfig); !ok {
		t.Fatal("expected dotnetConfig for netcore build")
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["rainbond.io/cnb-language"] != "dotnet" {
		t.Fatalf("expected dotnet debug annotation, got %q", annotations["rainbond.io/cnb-language"])
	}
	if annotations["cnb-bp-dotnet-framework-version"] != "8.0" {
		t.Fatalf("expected cnb-bp-dotnet-framework-version=8.0, got %q", annotations["cnb-bp-dotnet-framework-version"])
	}
	if annotations["cnb-bp-dotnet-project-path"] != "./src/WebApp" {
		t.Fatalf("expected cnb-bp-dotnet-project-path=./src/WebApp, got %q", annotations["cnb-bp-dotnet-project-path"])
	}
	if annotations["cnb-bp-dotnet-publish-flags"] != "--verbosity=normal" {
		t.Fatalf("expected cnb-bp-dotnet-publish-flags=--verbosity=normal, got %q", annotations["cnb-bp-dotnet-publish-flags"])
	}
}
