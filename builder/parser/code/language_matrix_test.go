package code

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for relativePath, content := range files {
		fullPath := filepath.Join(root, relativePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", fullPath, err)
		}
	}
}

// capability_id: rainbond.source-detect.language-matrix
func TestGetLangType_SupportedSourceBuildLanguages(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  Lang
	}{
		{name: "dockerfile", files: map[string]string{"Dockerfile": "FROM alpine\n"}, want: Dockerfile},
		{name: "java-maven", files: map[string]string{"pom.xml": "<project/>\n"}, want: JavaMaven},
		{name: "java-war", files: map[string]string{"demo.war": ""}, want: JaveWar},
		{name: "java-jar", files: map[string]string{"demo.jar": ""}, want: JavaJar},
		{name: "python", files: map[string]string{"requirements.txt": "flask==3.0.0\n"}, want: Python},
		{name: "php", files: map[string]string{"composer.json": "{}\n"}, want: PHP},
		{name: "go", files: map[string]string{"go.mod": "module example.com/demo\n\ngo 1.20\n"}, want: Golang},
		{name: "nodejs", files: map[string]string{"package.json": "{\"name\":\"demo\"}\n"}, want: Nodejs},
		{name: "static", files: map[string]string{"index.html": "<html></html>\n"}, want: Static},
		{name: "netcore", files: map[string]string{"demo.csproj": "<Project />\n"}, want: NetCore},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTestFiles(t, dir, tt.files)

			got, err := GetLangType(dir)
			if err != nil {
				t.Fatalf("GetLangType(%s) error = %v", tt.name, err)
			}
			if got != tt.want {
				t.Fatalf("GetLangType(%s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// capability_id: rainbond.source-detect.nodejs-over-static
func TestGetLangType_NodeJsWinsOverStaticWhenPackageJsonExists(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"package.json": "{\"name\":\"demo\"}\n",
		"index.html":   "<html></html>\n",
	})

	got, err := GetLangType(dir)
	if err != nil {
		t.Fatalf("GetLangType() error = %v", err)
	}
	if got != Nodejs {
		t.Fatalf("GetLangType() = %q, want %q", got, Nodejs)
	}
}

// capability_id: rainbond.source-detect.dockerfile-subdir
func TestGetLangType_DetectsDockerfileInSubDirectory(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"services/api/Dockerfile": "FROM alpine\n",
	})

	got, err := GetLangType(dir)
	if err != nil {
		t.Fatalf("GetLangType() error = %v", err)
	}
	if got != Dockerfile {
		t.Fatalf("GetLangType() = %q, want %q", got, Dockerfile)
	}
}
