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
			"BUILD_RUNTIMES":                  "8.2",
			"BUILD_RUNTIMES_SERVER":           "nginx",
			"BUILD_COMPOSER_VERSION":          "2.8.1",
			"BUILD_COMPOSER_INSTALL_OPTIONS":  "--no-dev",
			"BUILD_COMPOSER_INSTALL_GLOBAL":   "true",
			"BUILD_PHP_WEB_DIR":               "public",
			"BUILD_PHP_NGINX_ENABLE_HTTPS":    "true",
			"BUILD_PHP_ENABLE_HTTPS_REDIRECT": "true",
			"BUILD_COMPOSER_VENDOR_DIR":       "vendor-custom",
			"BUILD_COMPOSER_FILE":             "composer.custom.json",
			"BUILD_COMPOSER_AUTH":             "{\"http-basic\":{}}",
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
	if annotations["cnb-bp-composer-version"] != "2.8.1" {
		t.Fatalf("expected cnb-bp-composer-version, got %q", annotations["cnb-bp-composer-version"])
	}
	if annotations["cnb-bp-composer-install-options"] != "--no-dev" {
		t.Fatalf("expected cnb-bp-composer-install-options, got %q", annotations["cnb-bp-composer-install-options"])
	}
	if annotations["cnb-bp-composer-install-global"] != "true" {
		t.Fatalf("expected cnb-bp-composer-install-global, got %q", annotations["cnb-bp-composer-install-global"])
	}
	if annotations["cnb-bp-php-web-dir"] != "public" {
		t.Fatalf("expected cnb-bp-php-web-dir, got %q", annotations["cnb-bp-php-web-dir"])
	}
	if annotations["cnb-bp-php-nginx-enable-https"] != "true" {
		t.Fatalf("expected cnb-bp-php-nginx-enable-https, got %q", annotations["cnb-bp-php-nginx-enable-https"])
	}
	if annotations["cnb-bp-php-enable-https-redirect"] != "true" {
		t.Fatalf("expected cnb-bp-php-enable-https-redirect, got %q", annotations["cnb-bp-php-enable-https-redirect"])
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

func TestPHPBuilderRouting(t *testing.T) {
	if got := GetCNBBuilderImageForLanguage(code.PHP); got == DefaultCNBBuilder {
		t.Fatalf("expected php builder image to differ from noble default, got %q", got)
	}
	if got := GetCNBRunImageForLanguage(code.PHP); got == DefaultCNBRunImage {
		t.Fatalf("expected php run image to differ from noble default, got %q", got)
	}
}
