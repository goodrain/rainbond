package cnb

import (
	"os"
	"path/filepath"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
)

// staticConfig implements LanguageConfig for pure static projects (no package.json).
type staticConfig struct{}

// BuildAnnotations configures nginx web server for static file serving.
func (s *staticConfig) BuildAnnotations(re *build.Request, annotations map[string]string) {
	outputDir := re.BuildEnvs["CNB_OUTPUT_DIR"]
	if outputDir == "" {
		outputDir = "."
	}
	annotations["cnb-bp-web-server"] = "nginx"
	annotations["cnb-bp-web-server-root"] = outputDir
	annotations["cnb-bp-web-server-enable-push-state"] = "true"
	logrus.Infof("Pure static project: nginx web server at '%s'", outputDir)
}

// InjectMirrorConfig is a no-op for static projects (no package manager).
func (s *staticConfig) InjectMirrorConfig(re *build.Request) error {
	return nil
}

// CustomOrder returns nginx-only buildpack order for static projects.
func (s *staticConfig) CustomOrder(re *build.Request) []orderBuildpack {
	version := util.GetenvDefault("CNB_NGINX_BUILDPACK_VERSION", "1.0.12")
	return []orderBuildpack{
		{ID: "paketo-buildpacks/nginx", Version: version},
	}
}

// isPureStaticProject checks if the source directory has no package.json.
func isPureStaticProject(sourceDir string) bool {
	packageJsonPath := filepath.Join(sourceDir, "package.json")
	_, err := os.Stat(packageJsonPath)
	return os.IsNotExist(err)
}
