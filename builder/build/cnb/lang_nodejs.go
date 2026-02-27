package cnb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
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

	// Dependency mirror: check local offline marker first, then env, then default online URL.
	mirror := getDependencyMirror()
	annotations["cnb-bp-dependency-mirror"] = mirror

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

// defaultOnlineMirror is the public object storage URL for CNB dependencies.
const defaultOnlineMirror = "https://buildpack.rainbond.com/cnb"

// offlineMirrorMarker is the file path inside the build pod (grdata mount)
// that an offline provisioning tool writes to switch to local file:// mirror.
var offlineMirrorMarker = "/grdata/cnb/BP_DEPENDENCY_MIRROR"

// getDependencyMirror returns the CNB dependency mirror URL.
// Priority: env var > offline marker file > default online URL.
//
// For offline/air-gapped environments, write the mirror URL into the marker
// file at /opt/rainbond/grdata/cnb/BP_DEPENDENCY_MIRROR. Example content:
//
//	file://../../../../grdata/cnb
//
// Note: Paketo resolves file:// URIs relative to the buildpack install dir
// (/cnb/buildpacks/xxx/version/), so use ../../../../ to traverse back to /.
func getDependencyMirror() string {
	if v := os.Getenv("BP_DEPENDENCY_MIRROR"); v != "" {
		return v
	}
	if data, err := os.ReadFile(offlineMirrorMarker); err == nil {
		if m := strings.TrimSpace(string(data)); m != "" {
			logrus.Infof("Using dependency mirror from %s: %s", offlineMirrorMarker, m)
			return m
		}
	}
	return defaultOnlineMirror
}
