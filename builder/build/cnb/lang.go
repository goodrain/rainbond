package cnb

import "github.com/goodrain/rainbond/builder/build"

// LanguageConfig provides language-specific CNB build configuration.
// Each supported language implements this interface to customize
// annotations, mirror injection, and buildpack ordering.
type LanguageConfig interface {
	// BuildAnnotations adds language-specific BP_* annotations.
	BuildAnnotations(re *build.Request, annotations map[string]string)
	// InjectMirrorConfig injects language-specific mirror/proxy config files.
	InjectMirrorConfig(re *build.Request) error
	// CustomOrder returns custom buildpack order, or nil to use builder default.
	CustomOrder(re *build.Request) []orderBuildpack
}

// getLanguageConfig returns the LanguageConfig for the build request.
// Dispatches based on project type: pure static (no package.json) or Node.js.
// Future languages (Python, Go, etc.) will be added here.
func getLanguageConfig(re *build.Request) LanguageConfig {
	if isPureStaticProject(re.SourceDir) {
		return &staticConfig{}
	}
	return &nodejsConfig{}
}
