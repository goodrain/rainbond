package cnb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
)

func TestResolveExplicitVersionUsesPolicyAndNormalization(t *testing.T) {
	re := &build.Request{
		Lang:          code.Python,
		BuildStrategy: "cnb",
		BuildEnvs: map[string]string{
			"BUILD_RUNTIMES": "python-3.11.9",
		},
		CNBVersionPolicy: &build.CNBVersionPolicy{
			Version: 1,
			Languages: map[string]build.CNBLanguagePolicy{
				"python": {
					LangKey:         "python",
					AllowedVersions: []string{"3.10", "3.11"},
					DefaultVersion:  "3.10",
				},
			},
		},
	}

	if err := applyVersionPolicy(re); err != nil {
		t.Fatalf("applyVersionPolicy returned error: %v", err)
	}
	if got := re.BuildEnvs["BP_CPYTHON_VERSION"]; got != "3.11" {
		t.Fatalf("expected BP_CPYTHON_VERSION=3.11, got %q", got)
	}
	if got := re.BuildEnvs["BUILD_RUNTIMES"]; got != "3.11" {
		t.Fatalf("expected BUILD_RUNTIMES=3.11, got %q", got)
	}
}

func TestResolveExplicitVersionRejectsDisallowedVersion(t *testing.T) {
	re := &build.Request{
		Lang:          code.Nodejs,
		BuildStrategy: "cnb",
		BuildEnvs: map[string]string{
			"CNB_NODE_VERSION": "20",
		},
		CNBVersionPolicy: &build.CNBVersionPolicy{
			Version: 1,
			Languages: map[string]build.CNBLanguagePolicy{
				"nodejs": {
					LangKey:         "node",
					AllowedVersions: []string{"24.13.0"},
					DefaultVersion:  "24.13.0",
				},
			},
		},
	}

	err := applyVersionPolicy(re)
	if err == nil {
		t.Fatal("expected disallowed explicit version to fail")
	}
}

func TestResolveSourceDetectedVersionRejectsInsteadOfFallingBack(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "runtime.txt"), []byte("python-3.11.9"), 0644); err != nil {
		t.Fatalf("write runtime.txt: %v", err)
	}

	re := &build.Request{
		Lang:          code.Python,
		BuildStrategy: "cnb",
		SourceDir:     dir,
		BuildEnvs:     map[string]string{},
		CNBVersionPolicy: &build.CNBVersionPolicy{
			Version: 1,
			Languages: map[string]build.CNBLanguagePolicy{
				"python": {
					LangKey:         "python",
					AllowedVersions: []string{"3.10"},
					DefaultVersion:  "3.10",
				},
			},
		},
	}

	err := applyVersionPolicy(re)
	if err == nil {
		t.Fatal("expected disallowed source detected version to fail")
	}
}

func TestResolveDefaultVersionWhenNoExplicitOrSourceVersion(t *testing.T) {
	re := &build.Request{
		Lang:          code.JavaMaven,
		BuildStrategy: "cnb",
		SourceDir:     t.TempDir(),
		BuildEnvs:     map[string]string{},
		CNBVersionPolicy: &build.CNBVersionPolicy{
			Version: 1,
			Languages: map[string]build.CNBLanguagePolicy{
				"java": {
					LangKey:         "openJDK",
					AllowedVersions: []string{"11", "17"},
					DefaultVersion:  "17",
				},
			},
		},
	}

	if err := applyVersionPolicy(re); err != nil {
		t.Fatalf("applyVersionPolicy returned error: %v", err)
	}
	if got := re.BuildEnvs["BP_JVM_VERSION"]; got != "17" {
		t.Fatalf("expected BP_JVM_VERSION=17, got %q", got)
	}
}

func TestResolveOSSFallbackVersionWithoutPolicy(t *testing.T) {
	re := &build.Request{
		Lang:          code.Golang,
		BuildStrategy: "cnb",
		SourceDir:     t.TempDir(),
		BuildEnvs:     map[string]string{},
	}

	if err := applyVersionPolicy(re); err != nil {
		t.Fatalf("applyVersionPolicy returned error: %v", err)
	}
	if got := re.BuildEnvs["BP_GO_VERSION"]; got != "1.23" {
		t.Fatalf("expected BP_GO_VERSION=1.23, got %q", got)
	}
}
