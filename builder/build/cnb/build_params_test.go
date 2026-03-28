package cnb

import (
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
}
