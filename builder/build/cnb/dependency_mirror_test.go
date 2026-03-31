package cnb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
)

func TestBuildPlatformAnnotationsDependencyMirror_DefaultForAllLanguages(t *testing.T) {
	t.Setenv("BP_DEPENDENCY_MIRROR", "")

	origMarker := offlineMirrorMarker
	offlineMirrorMarker = filepath.Join(t.TempDir(), "missing-marker")
	defer func() { offlineMirrorMarker = origMarker }()

	for _, tt := range dependencyMirrorAnnotationCases(t) {
		t.Run(tt.name, func(t *testing.T) {
			annotations := (&Builder{}).buildPlatformAnnotations(tt.request)
			if annotations["cnb-bp-dependency-mirror"] != defaultOnlineMirror {
				t.Fatalf("expected default dependency mirror %q, got %q", defaultOnlineMirror, annotations["cnb-bp-dependency-mirror"])
			}
		})
	}
}

func TestBuildPlatformAnnotationsDependencyMirror_OfflineMarkerForAllLanguages(t *testing.T) {
	t.Setenv("BP_DEPENDENCY_MIRROR", "")

	dir := t.TempDir()
	marker := filepath.Join(dir, "BP_DEPENDENCY_MIRROR")
	if err := os.WriteFile(marker, []byte("file://../../../../grdata/cnb\n"), 0644); err != nil {
		t.Fatalf("write offline marker: %v", err)
	}

	origMarker := offlineMirrorMarker
	offlineMirrorMarker = marker
	defer func() { offlineMirrorMarker = origMarker }()

	for _, tt := range dependencyMirrorAnnotationCases(t) {
		t.Run(tt.name, func(t *testing.T) {
			annotations := (&Builder{}).buildPlatformAnnotations(tt.request)
			if annotations["cnb-bp-dependency-mirror"] != "file://../../../../grdata/cnb" {
				t.Fatalf("expected offline dependency mirror, got %q", annotations["cnb-bp-dependency-mirror"])
			}
		})
	}
}

type dependencyMirrorAnnotationCase struct {
	name    string
	request *build.Request
}

func dependencyMirrorAnnotationCases(t *testing.T) []dependencyMirrorAnnotationCase {
	t.Helper()

	nodeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(nodeDir, "package.json"), []byte("{\"name\":\"demo\"}\n"), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	staticDir := t.TempDir()

	return []dependencyMirrorAnnotationCase{
		{
			name: "nodejs",
			request: &build.Request{
				Lang:      code.Nodejs,
				SourceDir: nodeDir,
				BuildEnvs: map[string]string{},
			},
		},
		{
			name: "static",
			request: &build.Request{
				Lang:      code.Static,
				SourceDir: staticDir,
				BuildEnvs: map[string]string{},
			},
		},
		{
			name: "golang",
			request: &build.Request{
				Lang:      code.Golang,
				SourceDir: t.TempDir(),
				BuildEnvs: map[string]string{},
			},
		},
		{
			name: "python",
			request: &build.Request{
				Lang:      code.Python,
				SourceDir: t.TempDir(),
				BuildEnvs: map[string]string{},
			},
		},
		{
			name: "java",
			request: &build.Request{
				Lang:      code.JavaMaven,
				SourceDir: t.TempDir(),
				BuildEnvs: map[string]string{},
			},
		},
		{
			name: "php",
			request: &build.Request{
				Lang:      code.PHP,
				SourceDir: t.TempDir(),
				BuildEnvs: map[string]string{},
			},
		},
		{
			name: "dotnet",
			request: &build.Request{
				Lang:      code.NetCore,
				SourceDir: t.TempDir(),
				BuildEnvs: map[string]string{},
			},
		},
	}
}
