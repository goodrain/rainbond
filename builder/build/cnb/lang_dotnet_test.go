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
		BuildEnvs: map[string]string{},
	}

	if _, ok := getLanguageConfig(re).(*dotnetConfig); !ok {
		t.Fatal("expected dotnetConfig for netcore build")
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["rainbond.io/cnb-language"] != "dotnet" {
		t.Fatalf("expected dotnet debug annotation, got %q", annotations["rainbond.io/cnb-language"])
	}
}
