// RAINBOND, Application Management Platform
// Copyright (C) 2014-2018 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package types

// RuntimeInfo contains structured detection results for any language.
// This is separate from build environment variables (BUILD_*) which are
// passed to the actual build process.
type RuntimeInfo struct {
	// Language detected (e.g., "nodejs", "python", "java", "golang")
	Language string `json:"language,omitempty"`

	// LanguageVersion is the runtime version (e.g., "20.20.0" for Node.js)
	LanguageVersion string `json:"language_version,omitempty"`

	// VersionSource indicates where the version was detected from
	// e.g., "package.json", "runtime.txt", ".nvmrc", "default"
	VersionSource string `json:"version_source,omitempty"`

	// Framework detection results
	Framework *FrameworkInfo `json:"framework,omitempty"`

	// PackageManager detection results
	PackageManager *PackageManagerInfo `json:"package_manager,omitempty"`

	// BuildConfig recommended build configuration
	BuildConfig *BuildConfig `json:"build_config,omitempty"`

	// ConfigFiles detected configuration files
	ConfigFiles *ConfigFiles `json:"config_files,omitempty"`
}

// FrameworkInfo contains detected framework information.
// Applicable to all languages: Node.js (Next.js, Express), Python (Django, Flask),
// Java (Spring Boot), Go (Gin, Echo), etc.
type FrameworkInfo struct {
	// Name is the framework identifier (e.g., "nextjs", "express", "django", "spring-boot")
	Name string `json:"name"`

	// DisplayName is the human-readable name (e.g., "Next.js", "Express", "Django")
	DisplayName string `json:"display_name"`

	// Version of the framework if detected
	Version string `json:"version,omitempty"`

	// Type indicates the runtime type: "static", "dynamic", or "hybrid"
	// - static: builds to static files (e.g., Vite, CRA, Vue CLI)
	// - dynamic: requires a runtime server (e.g., Express, Django, Spring Boot)
	// - hybrid: supports both modes (e.g., Next.js with export)
	Type string `json:"type"`
}

// PackageManagerInfo contains package manager detection results.
type PackageManagerInfo struct {
	// Name of the package manager (e.g., "npm", "yarn", "pnpm", "pip", "maven", "gradle")
	Name string `json:"name"`

	// Version of the package manager if specified
	Version string `json:"version,omitempty"`

	// LockFile detected lock file name (e.g., "package-lock.json", "yarn.lock")
	LockFile string `json:"lock_file,omitempty"`
}

// BuildConfig contains recommended build configuration.
type BuildConfig struct {
	// OutputDir is the build output directory (e.g., "dist", "build", "target")
	OutputDir string `json:"output_dir,omitempty"`

	// BuildCommand is the recommended build command (e.g., "npm run build", "mvn package")
	BuildCommand string `json:"build_command,omitempty"`

	// StartCommand is the recommended start command (e.g., "npm start", "java -jar app.jar")
	StartCommand string `json:"start_command,omitempty"`

	// InstallCommand is the dependency install command (e.g., "npm install", "pip install -r requirements.txt")
	InstallCommand string `json:"install_command,omitempty"`
}

// ConfigFiles indicates which configuration files were detected.
type ConfigFiles struct {
	// HasNpmrc indicates .npmrc file exists
	HasNpmrc bool `json:"has_npmrc,omitempty"`

	// HasYarnrc indicates .yarnrc or .yarnrc.yml file exists
	HasYarnrc bool `json:"has_yarnrc,omitempty"`

	// HasPnpmrc indicates .pnpmrc file exists
	HasPnpmrc bool `json:"has_pnpmrc,omitempty"`

	// HasDockerfile indicates Dockerfile exists
	HasDockerfile bool `json:"has_dockerfile,omitempty"`

	// HasProcfile indicates Procfile exists
	HasProcfile bool `json:"has_procfile,omitempty"`
}

// NewRuntimeInfo creates a new RuntimeInfo with the given language.
func NewRuntimeInfo(language string) *RuntimeInfo {
	return &RuntimeInfo{
		Language: language,
	}
}

// WithFramework sets the framework information.
func (r *RuntimeInfo) WithFramework(name, displayName, version, runtimeType string) *RuntimeInfo {
	r.Framework = &FrameworkInfo{
		Name:        name,
		DisplayName: displayName,
		Version:     version,
		Type:        runtimeType,
	}
	return r
}

// WithPackageManager sets the package manager information.
func (r *RuntimeInfo) WithPackageManager(name, version, lockFile string) *RuntimeInfo {
	r.PackageManager = &PackageManagerInfo{
		Name:     name,
		Version:  version,
		LockFile: lockFile,
	}
	return r
}

// WithBuildConfig sets the build configuration.
func (r *RuntimeInfo) WithBuildConfig(outputDir, buildCmd, startCmd, installCmd string) *RuntimeInfo {
	r.BuildConfig = &BuildConfig{
		OutputDir:      outputDir,
		BuildCommand:   buildCmd,
		StartCommand:   startCmd,
		InstallCommand: installCmd,
	}
	return r
}

// IsStaticFramework returns true if the framework produces static output.
func (r *RuntimeInfo) IsStaticFramework() bool {
	return r.Framework != nil && r.Framework.Type == "static"
}

// IsDynamicFramework returns true if the framework requires a runtime server.
func (r *RuntimeInfo) IsDynamicFramework() bool {
	return r.Framework != nil && r.Framework.Type == "dynamic"
}
