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
		Lang:          code.JavaMaven,
		BuildStrategy: "cnb",
		SourceDir:     dir,
		BuildEnvs: map[string]string{
			"BP_JVM_VERSION":                      "17",
			"BP_JVM_TYPE":                         "JDK",
			"BP_MAVEN_BUILD_ARGUMENTS":            "clean package",
			"BP_MAVEN_ADDITIONAL_BUILD_ARGUMENTS": "-DskipTests",
			"BP_MAVEN_BUILT_MODULE":               "service-a",
			"BP_MAVEN_BUILT_ARTIFACT":             "service-a/target/app.jar",
			"BUILD_MAVEN_JAVA_OPTS":               "-Xmx1024m",
			"BUILD_PROCFILE":                      "web: java $JAVA_OPTS -jar target/app.jar",
		},
	}

	if _, ok := getLanguageConfig(re).(*javaConfig); !ok {
		t.Fatal("expected javaConfig for java build")
	}
	if err := validateSupportedBuildParams(re); err != nil {
		t.Fatalf("validateSupportedBuildParams returned error: %v", err)
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-jvm-version"] != "17" {
		t.Fatalf("expected cnb-bp-jvm-version=17, got %q", annotations["cnb-bp-jvm-version"])
	}
	if annotations["cnb-bp-jvm-type"] != "JDK" {
		t.Fatalf("expected cnb-bp-jvm-type=JDK, got %q", annotations["cnb-bp-jvm-type"])
	}
	if annotations["cnb-bp-maven-build-arguments"] != "clean package" {
		t.Fatalf("expected maven build arguments, got %q", annotations["cnb-bp-maven-build-arguments"])
	}
	if annotations["cnb-bp-maven-additional-build-arguments"] != "-DskipTests" {
		t.Fatalf("expected maven additional build arguments, got %q", annotations["cnb-bp-maven-additional-build-arguments"])
	}
	if annotations["cnb-bp-maven-version"] != "3.9.14" {
		t.Fatalf("expected fixed cnb-bp-maven-version=3.9.14, got %q", annotations["cnb-bp-maven-version"])
	}
	if annotations["cnb-bp-maven-built-module"] != "service-a" {
		t.Fatalf("expected cnb-bp-maven-built-module=service-a, got %q", annotations["cnb-bp-maven-built-module"])
	}
	if annotations["cnb-bp-maven-built-artifact"] != "service-a/target/app.jar" {
		t.Fatalf("expected cnb-bp-maven-built-artifact=service-a/target/app.jar, got %q", annotations["cnb-bp-maven-built-artifact"])
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
	wantEnv := map[string]string{
		"BP_MAVEN_BUILD_ARGUMENTS":            "clean package",
		"BP_MAVEN_ADDITIONAL_BUILD_ARGUMENTS": "-DskipTests",
		"BP_MAVEN_BUILT_MODULE":               "service-a",
		"BP_MAVEN_BUILT_ARTIFACT":             "service-a/target/app.jar",
		"MAVEN_OPTS":                          "-Xmx1024m",
	}
	found := map[string]bool{}
	for _, env := range envs {
		if wantValue, ok := wantEnv[env.Name]; ok {
			found[env.Name] = true
			if env.Value != wantValue {
				t.Fatalf("expected %s=%q, got %q", env.Name, wantValue, env.Value)
			}
		}
	}
	for name := range wantEnv {
		if !found[name] {
			t.Fatalf("expected %s env var for java cnb build", name)
		}
	}
}

func TestJavaLanguageConfigPrefersBPFieldsOverLegacyBuildFields(t *testing.T) {
	re := &build.Request{
		Lang:      code.JavaMaven,
		SourceDir: t.TempDir(),
		BuildEnvs: map[string]string{
			"BP_JVM_VERSION":                      "21",
			"BUILD_RUNTIMES":                      "17",
			"BP_MAVEN_BUILD_ARGUMENTS":            "verify",
			"BUILD_MAVEN_CUSTOM_GOALS":            "clean package",
			"BP_MAVEN_ADDITIONAL_BUILD_ARGUMENTS": "-Pprod",
			"BUILD_MAVEN_CUSTOM_OPTS":             "-DskipTests",
			"BP_MAVEN_BUILT_MODULE":               "bp-module",
			"BUILD_MAVEN_BUILT_MODULE":            "legacy-module",
			"BP_MAVEN_BUILT_ARTIFACT":             "bp-module/target/app.jar",
			"BUILD_MAVEN_BUILT_ARTIFACT":          "legacy-module/target/app.jar",
		},
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-jvm-version"] != "21" {
		t.Fatalf("expected BP_JVM_VERSION to win, got %q", annotations["cnb-bp-jvm-version"])
	}
	if annotations["cnb-bp-maven-build-arguments"] != "verify" {
		t.Fatalf("expected BP_MAVEN_BUILD_ARGUMENTS to win, got %q", annotations["cnb-bp-maven-build-arguments"])
	}
	if annotations["cnb-bp-maven-additional-build-arguments"] != "-Pprod" {
		t.Fatalf("expected BP_MAVEN_ADDITIONAL_BUILD_ARGUMENTS to win, got %q", annotations["cnb-bp-maven-additional-build-arguments"])
	}
	if annotations["cnb-bp-maven-built-module"] != "bp-module" {
		t.Fatalf("expected BP_MAVEN_BUILT_MODULE to win, got %q", annotations["cnb-bp-maven-built-module"])
	}
	if annotations["cnb-bp-maven-built-artifact"] != "bp-module/target/app.jar" {
		t.Fatalf("expected BP_MAVEN_BUILT_ARTIFACT to win, got %q", annotations["cnb-bp-maven-built-artifact"])
	}

	envs := (&Builder{}).buildEnvVars(re)
	wantEnv := map[string]string{
		"BP_MAVEN_BUILD_ARGUMENTS":            "verify",
		"BP_MAVEN_ADDITIONAL_BUILD_ARGUMENTS": "-Pprod",
		"BP_MAVEN_BUILT_MODULE":               "bp-module",
		"BP_MAVEN_BUILT_ARTIFACT":             "bp-module/target/app.jar",
	}
	found := map[string]string{}
	for _, env := range envs {
		found[env.Name] = env.Value
	}
	for name, want := range wantEnv {
		if got := found[name]; got != want {
			t.Fatalf("expected %s=%q, got %q", name, want, got)
		}
	}
}

func TestGradleLanguageConfigPrefersBPFieldsOverLegacyBuildFields(t *testing.T) {
	re := &build.Request{
		Lang:      code.Gradle,
		SourceDir: t.TempDir(),
		BuildEnvs: map[string]string{
			"BP_JVM_VERSION":                          "21",
			"BUILD_RUNTIMES":                          "17",
			"BP_GRADLE_BUILD_ARGUMENTS":               "assemble",
			"BUILD_GRADLE_BUILD_ARGUMENTS":            "build",
			"BP_GRADLE_ADDITIONAL_BUILD_ARGUMENTS":    "--info",
			"BUILD_GRADLE_ADDITIONAL_BUILD_ARGUMENTS": "--stacktrace",
			"BP_GRADLE_BUILT_MODULE":                  "bp-service",
			"BUILD_GRADLE_BUILT_MODULE":               "legacy-service",
			"BP_GRADLE_BUILT_ARTIFACT":                "bp-service/build/libs/app.jar",
			"BUILD_GRADLE_BUILT_ARTIFACT":             "legacy-service/build/libs/app.jar",
		},
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-jvm-version"] != "21" {
		t.Fatalf("expected BP_JVM_VERSION to win, got %q", annotations["cnb-bp-jvm-version"])
	}
	if annotations["cnb-bp-gradle-build-arguments"] != "assemble" {
		t.Fatalf("expected BP_GRADLE_BUILD_ARGUMENTS to win, got %q", annotations["cnb-bp-gradle-build-arguments"])
	}
	if annotations["cnb-bp-gradle-additional-build-arguments"] != "--info" {
		t.Fatalf("expected BP_GRADLE_ADDITIONAL_BUILD_ARGUMENTS to win, got %q", annotations["cnb-bp-gradle-additional-build-arguments"])
	}
	if annotations["cnb-bp-gradle-built-module"] != "bp-service" {
		t.Fatalf("expected BP_GRADLE_BUILT_MODULE to win, got %q", annotations["cnb-bp-gradle-built-module"])
	}
	if annotations["cnb-bp-gradle-built-artifact"] != "bp-service/build/libs/app.jar" {
		t.Fatalf("expected BP_GRADLE_BUILT_ARTIFACT to win, got %q", annotations["cnb-bp-gradle-built-artifact"])
	}
}

func TestJavaJarLanguageConfigExposesExecutableJarLocation(t *testing.T) {
	re := &build.Request{
		Lang:      code.JavaJar,
		SourceDir: t.TempDir(),
		BuildEnvs: map[string]string{
			"BP_EXECUTABLE_JAR_LOCATION": "target/app.jar",
		},
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-executable-jar-location"] != "target/app.jar" {
		t.Fatalf("expected executable jar location annotation, got %q", annotations["cnb-bp-executable-jar-location"])
	}
}

func TestJavaLanguageConfigAutoDetectsUniqueMavenModule(t *testing.T) {
	root := t.TempDir()
	writeFile := func(dir, content string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(content), 0644); err != nil {
			t.Fatalf("write pom.xml in %s: %v", dir, err)
		}
	}
	writeFile(root, `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>root</artifactId>
  <packaging>pom</packaging>
  <modules>
    <module>service-a</module>
  </modules>
</project>`)
	writeFile(filepath.Join(root, "service-a"), `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>service-a</artifactId>
  <packaging>jar</packaging>
</project>`)

	re := &build.Request{
		Lang:      code.JavaMaven,
		SourceDir: root,
		BuildEnvs: map[string]string{},
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-maven-built-module"] != "service-a" {
		t.Fatalf("expected unique maven module service-a, got %q", annotations["cnb-bp-maven-built-module"])
	}
	if annotations["cnb-bp-maven-built-artifact"] != "service-a/target/service-a-*.jar" {
		t.Fatalf("expected unique maven artifact, got %q", annotations["cnb-bp-maven-built-artifact"])
	}
}

func TestJavaLanguageConfigSkipsAmbiguousMavenModules(t *testing.T) {
	root := t.TempDir()
	writeFile := func(dir, content string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(content), 0644); err != nil {
			t.Fatalf("write pom.xml in %s: %v", dir, err)
		}
	}
	writeFile(root, `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>root</artifactId>
  <packaging>pom</packaging>
  <modules>
    <module>service-a</module>
    <module>service-b</module>
  </modules>
</project>`)
	writeFile(filepath.Join(root, "service-a"), `<project><modelVersion>4.0.0</modelVersion><groupId>com.example</groupId><artifactId>service-a</artifactId><packaging>jar</packaging></project>`)
	writeFile(filepath.Join(root, "service-b"), `<project><modelVersion>4.0.0</modelVersion><groupId>com.example</groupId><artifactId>service-b</artifactId><packaging>jar</packaging></project>`)

	re := &build.Request{
		Lang:      code.JavaMaven,
		SourceDir: root,
		BuildEnvs: map[string]string{},
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-maven-built-module"] != "" {
		t.Fatalf("expected ambiguous modules to skip auto-detect, got %q", annotations["cnb-bp-maven-built-module"])
	}
	if annotations["cnb-bp-maven-built-artifact"] != "" {
		t.Fatalf("expected ambiguous artifacts to skip auto-detect, got %q", annotations["cnb-bp-maven-built-artifact"])
	}
}

func TestGradleLanguageConfigAnnotations(t *testing.T) {
	re := &build.Request{
		Lang:      code.Gradle,
		SourceDir: t.TempDir(),
		BuildEnvs: map[string]string{
			"BUILD_RUNTIMES":                          "17",
			"BUILD_GRADLE_BUILD_ARGUMENTS":            "build",
			"BUILD_GRADLE_ADDITIONAL_BUILD_ARGUMENTS": "--stacktrace",
			"BUILD_GRADLE_BUILT_MODULE":               "service",
			"BUILD_GRADLE_BUILT_ARTIFACT":             "service/build/libs/app.jar",
		},
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-gradle-build-arguments"] != "build" {
		t.Fatalf("expected gradle build arguments, got %q", annotations["cnb-bp-gradle-build-arguments"])
	}
	if annotations["cnb-bp-gradle-additional-build-arguments"] != "--stacktrace" {
		t.Fatalf("expected gradle additional build arguments, got %q", annotations["cnb-bp-gradle-additional-build-arguments"])
	}
	if annotations["cnb-bp-gradle-built-module"] != "service" {
		t.Fatalf("expected gradle built module, got %q", annotations["cnb-bp-gradle-built-module"])
	}
	if annotations["cnb-bp-gradle-built-artifact"] != "service/build/libs/app.jar" {
		t.Fatalf("expected gradle built artifact, got %q", annotations["cnb-bp-gradle-built-artifact"])
	}
	if annotations["cnb-bp-jvm-version"] != "17" {
		t.Fatalf("expected cnb-bp-jvm-version=17, got %q", annotations["cnb-bp-jvm-version"])
	}
}
