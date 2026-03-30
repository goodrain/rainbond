package cnb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
)

func TestValidateSupportedBuildParamsPinsJavaMavenVersion(t *testing.T) {
	re := &build.Request{
		Lang:          code.JavaMaven,
		BuildStrategy: "cnb",
		BuildEnvs: map[string]string{
			"BUILD_RUNTIMES_MAVEN": "4",
			"BP_JAVA_APP_SERVER":   "tomcat-10",
		},
	}
	if err := validateSupportedBuildParams(re); err != nil {
		t.Fatalf("validateSupportedBuildParams returned error: %v", err)
	}
	if got := re.BuildEnvs["BUILD_RUNTIMES_MAVEN"]; got != "3.9.14" {
		t.Fatalf("expected BUILD_RUNTIMES_MAVEN to be pinned to 3.9.14, got %q", got)
	}
	if got := re.BuildEnvs["BP_JAVA_APP_SERVER"]; got != "tomcat" {
		t.Fatalf("expected BP_JAVA_APP_SERVER=tomcat, got %q", got)
	}
	if got := re.BuildEnvs["BUILD_RUNTIMES_SERVER"]; got != "tomcat" {
		t.Fatalf("expected BUILD_RUNTIMES_SERVER=tomcat, got %q", got)
	}
}

func TestValidateSupportedBuildParamsSetsDefaultJavaMavenVersion(t *testing.T) {
	re := &build.Request{
		Lang:          code.JavaMaven,
		BuildStrategy: "cnb",
		BuildEnvs:     map[string]string{},
	}
	if err := validateSupportedBuildParams(re); err != nil {
		t.Fatalf("validateSupportedBuildParams returned error: %v", err)
	}
	if got := re.BuildEnvs["BUILD_RUNTIMES_MAVEN"]; got != "3.9.14" {
		t.Fatalf("expected default BUILD_RUNTIMES_MAVEN=3.9.14, got %q", got)
	}
}

func TestValidateSupportedBuildParamsDefaultsJavaWarServer(t *testing.T) {
	re := &build.Request{
		Lang:          code.JaveWar,
		BuildStrategy: "cnb",
		BuildEnvs:     map[string]string{},
	}
	if err := validateSupportedBuildParams(re); err != nil {
		t.Fatalf("validateSupportedBuildParams returned error: %v", err)
	}
	if got := re.BuildEnvs["BP_JAVA_APP_SERVER"]; got != "tomcat" {
		t.Fatalf("expected default BP_JAVA_APP_SERVER=tomcat, got %q", got)
	}
}

func TestValidateSupportedBuildParamsRejectsUnknownPHPServer(t *testing.T) {
	re := &build.Request{
		Lang:          code.PHP,
		BuildStrategy: "cnb",
		BuildEnvs: map[string]string{
			"BUILD_RUNTIMES_SERVER": "iis",
		},
	}
	if err := validateSupportedBuildParams(re); err == nil {
		t.Fatal("expected unknown php server to fail validation")
	}
}

func TestValidateSupportedBuildParamsDefaultsPHPServerAndComposerVersion(t *testing.T) {
	re := &build.Request{
		Lang:          code.PHP,
		BuildStrategy: "cnb",
		BuildEnvs:     map[string]string{},
	}
	if err := validateSupportedBuildParams(re); err != nil {
		t.Fatalf("validateSupportedBuildParams returned error: %v", err)
	}
	if got := re.BuildEnvs["BUILD_RUNTIMES_SERVER"]; got != "nginx" {
		t.Fatalf("expected BUILD_RUNTIMES_SERVER=nginx, got %q", got)
	}
	if got := re.BuildEnvs["BP_COMPOSER_VERSION"]; got != "2.7.9" {
		t.Fatalf("expected BP_COMPOSER_VERSION=2.7.9, got %q", got)
	}
	if got := re.BuildEnvs["BUILD_COMPOSER_VERSION"]; got != "2.7.9" {
		t.Fatalf("expected BUILD_COMPOSER_VERSION=2.7.9, got %q", got)
	}
}

func TestValidateSupportedBuildParamsSynthesizesPHPBPKeys(t *testing.T) {
	re := &build.Request{
		Lang:          code.PHP,
		BuildStrategy: "cnb",
		BuildEnvs: map[string]string{
			"BUILD_COMPOSER_INSTALL_OPTIONS": "--no-dev",
			"BUILD_PHP_WEB_DIR":              "public",
		},
	}
	if err := validateSupportedBuildParams(re); err != nil {
		t.Fatalf("validateSupportedBuildParams returned error: %v", err)
	}
	if got := re.BuildEnvs["BP_COMPOSER_INSTALL_OPTIONS"]; got != "--no-dev" {
		t.Fatalf("expected BP_COMPOSER_INSTALL_OPTIONS=--no-dev, got %q", got)
	}
	if got := re.BuildEnvs["BP_PHP_WEB_DIR"]; got != "public" {
		t.Fatalf("expected BP_PHP_WEB_DIR=public, got %q", got)
	}
}

func TestValidateSupportedBuildParamsDetectsPythonManager(t *testing.T) {
	dir := t.TempDir()
	re := &build.Request{
		Lang:          code.Python,
		BuildStrategy: "cnb",
		SourceDir:     dir,
		BuildEnvs: map[string]string{
			"BUILD_PYTHON_PACKAGE_MANAGER":         "pipenv",
			"BUILD_PYTHON_PACKAGE_MANAGER_VERSION": "2024.4.1",
		},
	}
	if err := validateSupportedBuildParams(re); err != nil {
		t.Fatalf("validateSupportedBuildParams returned error: %v", err)
	}
	if got := re.BuildEnvs["BUILD_PYTHON_PACKAGE_MANAGER"]; got != "pipenv" {
		t.Fatalf("expected BUILD_PYTHON_PACKAGE_MANAGER=pipenv, got %q", got)
	}
	if got := re.BuildEnvs["BP_PIPENV_VERSION"]; got != "2024.4.1" {
		t.Fatalf("expected BP_PIPENV_VERSION to be synthesized, got %q", got)
	}
}

func TestValidateSupportedBuildParamsDetectsPythonManagerFromFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Pipfile"), []byte("[packages]\nflask='*'\n"), 0644); err != nil {
		t.Fatalf("write Pipfile: %v", err)
	}
	re := &build.Request{
		Lang:          code.Python,
		BuildStrategy: "cnb",
		SourceDir:     dir,
		BuildEnvs:     map[string]string{},
	}
	if err := validateSupportedBuildParams(re); err != nil {
		t.Fatalf("validateSupportedBuildParams returned error: %v", err)
	}
	if got := re.BuildEnvs["BUILD_PYTHON_PACKAGE_MANAGER"]; got != "pipenv" {
		t.Fatalf("expected auto-detected package manager pipenv, got %q", got)
	}
}

func TestValidateSupportedBuildParamsDefaultsCondaSolver(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "environment.yml"), []byte("name: demo\n"), 0644); err != nil {
		t.Fatalf("write environment.yml: %v", err)
	}
	re := &build.Request{
		Lang:          code.Python,
		BuildStrategy: "cnb",
		SourceDir:     dir,
		BuildEnvs:     map[string]string{},
	}
	if err := validateSupportedBuildParams(re); err != nil {
		t.Fatalf("validateSupportedBuildParams returned error: %v", err)
	}
	if got := re.BuildEnvs["BUILD_PYTHON_PACKAGE_MANAGER"]; got != "conda" {
		t.Fatalf("expected auto-detected package manager conda, got %q", got)
	}
	if got := re.BuildEnvs["BP_CONDA_SOLVER"]; got != "mamba" {
		t.Fatalf("expected default BP_CONDA_SOLVER=mamba, got %q", got)
	}
}

func TestResolvePlatformBindings(t *testing.T) {
	t.Run("uses explicit maven setting", func(t *testing.T) {
		ctrl := &mockJobCtrl{
			languageBuildSettings: map[string]string{
				"team-maven": "team-maven",
			},
		}
		builder := newTestBuilder(ctrl)
		re := &build.Request{
			Lang: code.JavaMaven,
			BuildEnvs: map[string]string{
				"BUILD_MAVEN_SETTING_NAME": "team-maven",
			},
		}
		bindings, err := builder.resolvePlatformBindings(re)
		if err != nil {
			t.Fatalf("resolvePlatformBindings returned error: %v", err)
		}
		if len(bindings) != 1 || bindings[0].ConfigMapName != "team-maven" {
			t.Fatalf("unexpected bindings: %+v", bindings)
		}
		if got := re.BuildEnvs["BP_MAVEN_SETTINGS_PATH"]; got != "/platform/bindings/team-maven/settings.xml" {
			t.Fatalf("expected BP_MAVEN_SETTINGS_PATH to match mounted binding path, got %q", got)
		}
	})

	t.Run("uses default maven setting when explicit value is empty", func(t *testing.T) {
		ctrl := &mockJobCtrl{
			defaultLanguageBuildSetting: "default-maven",
		}
		builder := newTestBuilder(ctrl)
		re := &build.Request{Lang: code.JavaMaven, BuildEnvs: map[string]string{}}
		bindings, err := builder.resolvePlatformBindings(re)
		if err != nil {
			t.Fatalf("resolvePlatformBindings returned error: %v", err)
		}
		if len(bindings) != 1 || bindings[0].ConfigMapName != "default-maven" {
			t.Fatalf("unexpected bindings: %+v", bindings)
		}
		if got := re.BuildEnvs["BP_MAVEN_SETTINGS_PATH"]; got != "/platform/bindings/default-maven/settings.xml" {
			t.Fatalf("expected BP_MAVEN_SETTINGS_PATH to match mounted binding path, got %q", got)
		}
	})

	t.Run("uses explicit nuget config for dotnet", func(t *testing.T) {
		ctrl := &mockJobCtrl{
			languageBuildSettings: map[string]string{
				"nuget-private": "nuget-private",
			},
		}
		builder := newTestBuilder(ctrl)
		re := &build.Request{
			Lang: code.NetCore,
			BuildEnvs: map[string]string{
				"BUILD_NUGET_CONFIG_NAME": "nuget-private",
			},
		}
		bindings, err := builder.resolvePlatformBindings(re)
		if err != nil {
			t.Fatalf("resolvePlatformBindings returned error: %v", err)
		}
		if len(bindings) != 1 || bindings[0].ConfigMapName != "nuget-private" {
			t.Fatalf("unexpected bindings: %+v", bindings)
		}
		if bindings[0].Type != "nugetconfig" {
			t.Fatalf("expected nugetconfig binding type, got %q", bindings[0].Type)
		}
		if bindings[0].ConfigMapKey != "nuget.config" || bindings[0].TargetFile != "nuget.config" {
			t.Fatalf("unexpected nuget binding files: %+v", bindings[0])
		}
	})
}
