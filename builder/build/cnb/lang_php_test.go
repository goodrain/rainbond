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
			"BP_PHP_VERSION":              "8.4",
			"BUILD_RUNTIMES_SERVER":       "nginx",
			"BP_COMPOSER_VERSION":         "2.7.9",
			"BP_COMPOSER_INSTALL_OPTIONS": "--no-dev",
			"BP_PHP_WEB_DIR":              "public",
			"BUILD_COMPOSER_VENDOR_DIR":   "vendor-custom",
			"BUILD_COMPOSER_FILE":         "composer.custom.json",
			"BUILD_COMPOSER_AUTH":         "{\"http-basic\":{}}",
		},
	}

	if _, ok := getLanguageConfig(re).(*phpConfig); !ok {
		t.Fatal("expected phpConfig for php build")
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if annotations["cnb-bp-php-version"] != "8.4" {
		t.Fatalf("expected cnb-bp-php-version=8.4, got %q", annotations["cnb-bp-php-version"])
	}
	if annotations["cnb-bp-php-server"] != "nginx" {
		t.Fatalf("expected cnb-bp-php-server=nginx, got %q", annotations["cnb-bp-php-server"])
	}
	if annotations["cnb-bp-composer-version"] != "2.7.9" {
		t.Fatalf("expected cnb-bp-composer-version, got %q", annotations["cnb-bp-composer-version"])
	}
	if annotations["cnb-bp-composer-install-options"] != "--no-dev" {
		t.Fatalf("expected cnb-bp-composer-install-options, got %q", annotations["cnb-bp-composer-install-options"])
	}
	if annotations["cnb-bp-php-web-dir"] != "public" {
		t.Fatalf("expected cnb-bp-php-web-dir, got %q", annotations["cnb-bp-php-web-dir"])
	}
	if annotations["rainbond.io/cnb-language"] != "php" {
		t.Fatalf("expected php debug annotation, got %q", annotations["rainbond.io/cnb-language"])
	}

	envs := (&Builder{}).buildEnvVars(re)
	gotEnv := map[string]string{}
	for _, env := range envs {
		gotEnv[env.Name] = env.Value
	}
	if gotEnv["COMPOSER_VENDOR_DIR"] != "vendor-custom" {
		t.Fatalf("expected COMPOSER_VENDOR_DIR, got %q", gotEnv["COMPOSER_VENDOR_DIR"])
	}
	if gotEnv["COMPOSER"] != "composer.custom.json" {
		t.Fatalf("expected COMPOSER, got %q", gotEnv["COMPOSER"])
	}
	if gotEnv["COMPOSER_AUTH"] != "{\"http-basic\":{}}" {
		t.Fatalf("expected COMPOSER_AUTH, got %q", gotEnv["COMPOSER_AUTH"])
	}
}

func TestPHPLanguageConfigDisablesHTTPSRedirectByDefault(t *testing.T) {
	re := &build.Request{
		Lang:          code.PHP,
		BuildStrategy: "cnb",
		SourceDir:     t.TempDir(),
		BuildEnvs:     map[string]string{},
	}

	if err := validateSupportedBuildParams(re); err != nil {
		t.Fatalf("validateSupportedBuildParams returned error: %v", err)
	}

	annotations := (&Builder{}).buildPlatformAnnotations(re)
	if got := annotations["cnb-bp-php-enable-https-redirect"]; got != "false" {
		t.Fatalf("expected cnb-bp-php-enable-https-redirect=false, got %q", got)
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
