package cnb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// Node.js mirror defaults
const (
	DefaultNpmrcContent = `registry=https://registry.npmmirror.com
`
	DefaultYarnrcContent = `registry "https://registry.npmmirror.com"
`
)

// serverFrameworks lists Node.js frameworks that run as server processes (no nginx).
var serverFrameworks = map[string]bool{
	"express": true, "koa": true, "nestjs": true,
	"nextjs": true, "nuxt": true, "other-server": true,
}

// nodejsConfig implements LanguageConfig for Node.js projects.
type nodejsConfig struct{}

// BuildAnnotations adds Node.js specific BP_* annotations.
func (n *nodejsConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	// NODE_ENV
	nodeEnv := "production"
	if v, ok := re.BuildEnvs["CNB_NODE_ENV"]; ok && v != "" {
		nodeEnv = v
	}
	annotations["cnb-node-env"] = nodeEnv

	// Node.js version
	if v, ok := re.BuildEnvs["CNB_NODE_VERSION"]; ok && v != "" {
		annotations["cnb-bp-node-version"] = v
	} else if v, ok := re.BuildEnvs["RUNTIMES"]; ok && v != "" {
		annotations["cnb-bp-node-version"] = v
	}

	applyDependencyMirrorAnnotation(annotations)

	// Registry for global npm tooling. The project .npmrc cannot cover this,
	// see resolveNpmRegistry.
	setAnnotationValue(annotations, "cnb-npm-config-registry", resolveNpmRegistry(re))

	// Build script
	if v, ok := re.BuildEnvs["CNB_BUILD_SCRIPT"]; ok && v != "" {
		annotations["cnb-bp-node-run-scripts"] = v
	}

	// Start script: CNB_START_SCRIPT → BP_NPM_START_SCRIPT
	if v, ok := re.BuildEnvs["CNB_START_SCRIPT"]; ok && v != "" {
		annotations["cnb-bp-npm-start-script"] = v
	}

	// Web server: framework determines frontend (nginx) vs backend (no nginx).
	// Server frameworks (nextjs, nuxt, express, etc.) run their own process.
	// Static/export variants (nextjs-static, nuxt-static, vite, cra, etc.) use nginx.
	framework := re.BuildEnvs["CNB_FRAMEWORK"]
	if framework != "" {
		isServer := serverFrameworks[framework]
		if !isServer {
			outputDir := re.BuildEnvs["CNB_OUTPUT_DIR"]
			if outputDir == "" {
				outputDir = "dist"
			}
			annotations["cnb-bp-web-server"] = "nginx"
			annotations["cnb-bp-web-server-root"] = outputDir
			annotations["cnb-bp-web-server-enable-push-state"] = "true"
		}
	} else if outputDir, ok := re.BuildEnvs["CNB_OUTPUT_DIR"]; ok && outputDir != "" {
		// Backward compatibility: no framework, fall back to CNB_OUTPUT_DIR
		annotations["cnb-bp-web-server"] = "nginx"
		annotations["cnb-bp-web-server-root"] = outputDir
		annotations["cnb-bp-web-server-enable-push-state"] = "true"
	}
}

func (n *nodejsConfig) BuildEnvVars(re *build.Request) []corev1.EnvVar {
	return nil
}

// InjectMirrorConfig injects .npmrc and .yarnrc for npm/yarn/pnpm registry configuration.
func (n *nodejsConfig) InjectMirrorConfig(re *build.Request) error {
	packageJsonPath := filepath.Join(re.SourceDir, "package.json")
	if _, err := os.Stat(packageJsonPath); os.IsNotExist(err) {
		logrus.Info("No package.json found, skipping package manager config injection")
		return nil
	}

	if re.BuildEnvs["CNB_MIRROR_SOURCE"] == "project" {
		for _, file := range []string{".npmrc", ".yarnrc"} {
			if _, err := os.Stat(filepath.Join(re.SourceDir, file)); err == nil {
				logrus.Infof("Using project config file: %s", file)
				return nil
			}
		}
		logrus.Info("No project config files found, using platform default configuration")
	}

	if err := injectConfigFile(re, ".npmrc", "CNB_MIRROR_NPMRC"); err != nil {
		return fmt.Errorf("inject .npmrc: %w", err)
	}
	if err := injectConfigFile(re, ".yarnrc", "CNB_MIRROR_YARNRC"); err != nil {
		return fmt.Errorf("inject .yarnrc: %w", err)
	}
	return nil
}

// CustomOrder returns nil — Node.js projects use the default builder order.
func (n *nodejsConfig) CustomOrder(re *build.Request) []orderBuildpack {
	return nil
}

// resolveNpmRegistry returns the registry npm should use for global installs.
//
// The pnpm buildpack bootstraps itself with `npm install -g pnpm@<version>`, and
// npm deliberately ignores the per-project .npmrc in global mode. So the mirror
// configured for the project — whether written by InjectMirrorConfig or committed
// by the user — never applies to that bootstrap, which then falls back to
// registry.npmjs.org. That fails on China networks and is unreachable in
// air-gapped clusters, even though the project mirror itself is fine.
//
// Exporting NPM_CONFIG_REGISTRY closes the gap: environment variables are the one
// config source npm still honours in global mode. This mirrors how Go builds
// receive GOPROXY (see lang_golang.go).
//
// Priority: CNB_NPM_REGISTRY > project .npmrc > CNB_MIRROR_NPMRC > China mirror default.
// The project .npmrc is preferred over any platform default because npm ranks
// environment variables *above* the project .npmrc: a value that disagreed with the
// project would silently redirect the whole install, not just the bootstrap.
func resolveNpmRegistry(re *build.Request) string {
	if registry := firstNonEmptyEnv(re.BuildEnvs, "CNB_NPM_REGISTRY", "BUILD_NPM_REGISTRY"); registry != "" {
		return strings.TrimSpace(registry)
	}
	if registry := npmRegistryFromNpmrc(readProjectNpmrc(re)); registry != "" {
		return registry
	}
	if registry := npmRegistryFromNpmrc(re.BuildEnvs["CNB_MIRROR_NPMRC"]); registry != "" {
		return registry
	}
	if util.GetenvDefault("ENABLE_CHINA_MIRROR", "true") != "true" {
		return ""
	}
	return npmRegistryFromNpmrc(util.GetenvDefault("DEFAULT_NPMRC", DefaultNpmrcContent))
}

// readProjectNpmrc reads the .npmrc that the Node.js buildpacks will see.
// BP_NODE_PROJECT_PATH moves the project root into a subdirectory, so look
// there first and fall back to the source root.
func readProjectNpmrc(re *build.Request) string {
	if re.SourceDir == "" {
		return ""
	}
	dirs := make([]string, 0, 2)
	if sub := strings.TrimSpace(re.BuildEnvs["BP_NODE_PROJECT_PATH"]); sub != "" {
		dirs = append(dirs, filepath.Join(re.SourceDir, sub))
	}
	dirs = append(dirs, re.SourceDir)

	for _, dir := range dirs {
		if data, err := os.ReadFile(filepath.Join(dir, ".npmrc")); err == nil {
			return string(data)
		}
	}
	return ""
}

// npmRegistryFromNpmrc extracts the default `registry=` value from .npmrc content.
// Scoped entries (`@scope:registry=`) are ignored: they do not apply to the
// unscoped packages installed during bootstrap.
func npmRegistryFromNpmrc(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "registry" {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return ""
}

// injectConfigFile injects a single config file (.npmrc or .yarnrc).
func injectConfigFile(re *build.Request, fileName, envKey string) error {
	filePath := filepath.Join(re.SourceDir, fileName)

	if _, err := os.Stat(filePath); err == nil {
		logrus.Infof("Config file %s already exists in project, skipping", fileName)
		return nil
	}

	if customContent, ok := re.BuildEnvs[envKey]; ok && customContent != "" {
		logrus.Infof("Using user-provided %s configuration from %s", fileName, envKey)
		return os.WriteFile(filePath, []byte(customContent), 0644)
	}

	if util.GetenvDefault("ENABLE_CHINA_MIRROR", "true") != "true" {
		return nil
	}

	var defaultContent string
	switch fileName {
	case ".npmrc":
		defaultContent = util.GetenvDefault("DEFAULT_NPMRC", DefaultNpmrcContent)
	case ".yarnrc":
		defaultContent = util.GetenvDefault("DEFAULT_YARNRC", DefaultYarnrcContent)
	default:
		return nil
	}

	if defaultContent != "" {
		logrus.Infof("Creating default %s with China mirror configuration", fileName)
		return os.WriteFile(filePath, []byte(defaultContent), 0644)
	}
	return nil
}
