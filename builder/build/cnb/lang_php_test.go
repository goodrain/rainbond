package cnb

import (
	"testing"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
)

func TestPHPLanguageConfigAnnotations(t *testing.T) {
	re := &build.Request{
		Lang:      code.PHP,
		SourceDir: t.TempDir(),
		BuildEnvs: map[string]string{
			"BUILD_RUNTIMES":        "8.2",
			"BUILD_RUNTIMES_SERVER": "nginx",
		},
	}

	if _, ok := getLanguageConfig(re).(*phpConfig); !ok {
		t.Fatal("expected phpConfig for php build")
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-php-version"] != "8.2" {
		t.Fatalf("expected cnb-bp-php-version=8.2, got %q", annotations["cnb-bp-php-version"])
	}
	if annotations["cnb-bp-php-server"] != "nginx" {
		t.Fatalf("expected cnb-bp-php-server=nginx, got %q", annotations["cnb-bp-php-server"])
	}
	if annotations["rainbond.io/cnb-language"] != "php" {
		t.Fatalf("expected php debug annotation, got %q", annotations["rainbond.io/cnb-language"])
	}
}

func TestPHPBuilderRouting(t *testing.T) {
	if got := GetCNBBuilderImageForLanguage(code.PHP); got == DefaultCNBBuilder {
		t.Fatalf("expected php builder image to differ from noble default, got %q", got)
	}
	if got := GetCNBRunImageForLanguage(code.PHP); got == DefaultCNBRunImage {
		t.Fatalf("expected php run image to differ from noble default, got %q", got)
	}
}
