package cnb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
)

func TestJavaLanguageConfigAnnotationsAndProcfile(t *testing.T) {
	dir := t.TempDir()
	re := &build.Request{
		Lang:      code.JavaMaven,
		SourceDir: dir,
		BuildEnvs: map[string]string{
			"BUILD_RUNTIMES":           "17",
			"BUILD_MAVEN_CUSTOM_GOALS": "clean package",
			"BUILD_MAVEN_CUSTOM_OPTS":  "-DskipTests",
			"BUILD_MAVEN_JAVA_OPTS":    "-Xmx1024m",
			"BUILD_PROCFILE":           "web: java $JAVA_OPTS -jar target/app.jar",
		},
	}

	if _, ok := getLanguageConfig(re).(*javaConfig); !ok {
		t.Fatal("expected javaConfig for java build")
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-jvm-version"] != "17" {
		t.Fatalf("expected cnb-bp-jvm-version=17, got %q", annotations["cnb-bp-jvm-version"])
	}
	if annotations["cnb-bp-maven-build-arguments"] != "clean package -DskipTests" {
		t.Fatalf("expected merged maven arguments, got %q", annotations["cnb-bp-maven-build-arguments"])
	}
	if annotations["rainbond.io/cnb-language"] != "java" {
		t.Fatalf("expected java debug annotation, got %q", annotations["rainbond.io/cnb-language"])
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
	if string(content) != "web: java $JAVA_OPTS -jar target/app.jar\n" {
		t.Fatalf("unexpected Procfile content %q", string(content))
	}

	envs := (&Builder{}).buildEnvVars(re)
	found := false
	for _, env := range envs {
		if env.Name == "MAVEN_OPTS" && env.Value == "-Xmx1024m" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected MAVEN_OPTS env var for java cnb build")
	}
}
