// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package code

import (
	"os"
	"path"

	simplejson "github.com/bitly/go-simplejson"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
)

// Framework represents detected Node.js framework information
type Framework struct {
	Name        string // nextjs, nuxt, umi, vite, cra, vue-cli, gatsby, remix, express, nestjs
	DisplayName string // Next.js, Nuxt, Umi...
	Version     string // 14.2.3
	RuntimeType string // static | dynamic
	OutputDir   string // default output directory
	BuildCmd    string // package.json script name (e.g., "build"), used for BP_NODE_RUN_SCRIPTS
	StartCmd    string // package.json script name (e.g., "start"), used for BP_NODE_RUN_SCRIPTS
}

// frameworkDetector defines detection rules for each framework
type frameworkDetector struct {
	name        string
	displayName string
	packages    []string
	configFiles []string
	runtimeType string
	outputDir   string
	buildCmd    string
	startCmd    string
}

// frameworkDetectors ordered by specificity (high to low)
var frameworkDetectors = []frameworkDetector{
	{
		name:        "nextjs",
		displayName: "Next.js",
		packages:    []string{"next"},
		configFiles: []string{"next.config.js", "next.config.mjs", "next.config.ts"},
		runtimeType: "dynamic",
		outputDir:   ".next",
		buildCmd:    "build", // CNB BP_NODE_RUN_SCRIPTS expects script name only
		startCmd:    "start",
	},
	{
		name:        "nuxt",
		displayName: "Nuxt",
		packages:    []string{"nuxt", "nuxt3"},
		configFiles: []string{"nuxt.config.js", "nuxt.config.ts"},
		runtimeType: "dynamic",
		outputDir:   ".nuxt",
		buildCmd:    "build",
		startCmd:    "start",
	},
	{
		name:        "umi",
		displayName: "Umi",
		packages:    []string{"umi", "@umijs/max"},
		configFiles: []string{".umirc.ts", ".umirc.js", "config/config.ts"},
		runtimeType: "static",
		outputDir:   "dist",
		buildCmd:    "build",
		startCmd:    "",
	},
	{
		name:        "remix",
		displayName: "Remix",
		packages:    []string{"@remix-run/node", "@remix-run/react"},
		configFiles: []string{"remix.config.js"},
		runtimeType: "dynamic",
		outputDir:   "",
		buildCmd:    "build",
		startCmd:    "start",
	},
	{
		name:        "gatsby",
		displayName: "Gatsby",
		packages:    []string{"gatsby"},
		configFiles: []string{"gatsby-config.js", "gatsby-config.ts"},
		runtimeType: "static",
		outputDir:   "public",
		buildCmd:    "build",
		startCmd:    "",
	},
	{
		name:        "docusaurus",
		displayName: "Docusaurus",
		packages:    []string{"@docusaurus/core"},
		configFiles: []string{"docusaurus.config.js", "docusaurus.config.ts"},
		runtimeType: "static",
		outputDir:   "build",
		buildCmd:    "build",
		startCmd:    "",
	},
	{
		name:        "vite",
		displayName: "Vite",
		packages:    []string{"vite"},
		configFiles: []string{"vite.config.js", "vite.config.ts"},
		runtimeType: "static",
		outputDir:   "dist",
		buildCmd:    "build",
		startCmd:    "",
	},
	{
		name:        "cra",
		displayName: "Create React App",
		packages:    []string{"react-scripts"},
		configFiles: nil,
		runtimeType: "static",
		outputDir:   "build",
		buildCmd:    "build",
		startCmd:    "",
	},
	{
		name:        "vue-cli",
		displayName: "Vue CLI",
		packages:    []string{"@vue/cli-service"},
		configFiles: []string{"vue.config.js"},
		runtimeType: "static",
		outputDir:   "dist",
		buildCmd:    "build",
		startCmd:    "",
	},
	{
		name:        "nestjs",
		displayName: "Nest.js",
		packages:    []string{"@nestjs/core"},
		configFiles: []string{"nest-cli.json"},
		runtimeType: "dynamic",
		outputDir:   "dist",
		buildCmd:    "build",
		startCmd:    "start:prod",
	},
	{
		name:        "express",
		displayName: "Express",
		packages:    []string{"express"},
		configFiles: nil,
		runtimeType: "dynamic",
		outputDir:   "",
		buildCmd:    "",
		startCmd:    "start",
	},
}

// DetectFramework detects Node.js framework from project source
func DetectFramework(buildPath string) *Framework {
	// 1. Read package.json
	pkgJSON := readPackageJSON(buildPath)
	if pkgJSON == nil {
		return nil
	}

	// 2. Iterate detectors by priority
	for _, detector := range frameworkDetectors {
		// Check if any dependency package exists
		if hasAnyDependency(pkgJSON, detector.packages) {
			// Check config files (if defined)
			if len(detector.configFiles) == 0 || hasAnyFile(buildPath, detector.configFiles) {
				return &Framework{
					Name:        detector.name,
					DisplayName: detector.displayName,
					Version:     getDependencyVersion(pkgJSON, detector.packages[0]),
					RuntimeType: detector.runtimeType,
					OutputDir:   detector.outputDir,
					BuildCmd:    detector.buildCmd,
					StartCmd:    detector.startCmd,
				}
			}
		}
	}

	return nil // No framework detected
}

// readPackageJSON reads and parses package.json file
func readPackageJSON(buildPath string) *simplejson.Json {
	packageJSONPath := path.Join(buildPath, "package.json")
	if ok, _ := util.FileExists(packageJSONPath); !ok {
		logrus.Debugf("package.json not found at %s", packageJSONPath)
		return nil
	}

	body, err := os.ReadFile(packageJSONPath)
	if err != nil {
		logrus.Warnf("Failed to read package.json at %s: %v", packageJSONPath, err)
		return nil
	}

	json, err := simplejson.NewJson(body)
	if err != nil {
		logrus.Warnf("Failed to parse package.json at %s: %v", packageJSONPath, err)
		return nil
	}

	return json
}

// hasAnyDependency checks if package.json contains any of the specified packages
func hasAnyDependency(pkgJSON *simplejson.Json, packages []string) bool {
	for _, pkg := range packages {
		if hasDependency(pkgJSON, pkg) {
			return true
		}
	}
	return false
}

// hasDependency checks if a specific package exists in dependencies or devDependencies
func hasDependency(pkgJSON *simplejson.Json, packageName string) bool {
	// Check dependencies
	if deps := pkgJSON.Get("dependencies"); deps != nil {
		if dep := deps.Get(packageName); dep != nil {
			if _, err := dep.String(); err == nil {
				return true
			}
		}
	}

	// Check devDependencies
	if devDeps := pkgJSON.Get("devDependencies"); devDeps != nil {
		if dep := devDeps.Get(packageName); dep != nil {
			if _, err := dep.String(); err == nil {
				return true
			}
		}
	}

	return false
}

// getDependencyVersion gets the version of a specific package
func getDependencyVersion(pkgJSON *simplejson.Json, packageName string) string {
	// Check dependencies first
	if deps := pkgJSON.Get("dependencies"); deps != nil {
		if dep := deps.Get(packageName); dep != nil {
			if version, err := dep.String(); err == nil {
				return cleanVersion(version)
			}
		}
	}

	// Check devDependencies
	if devDeps := pkgJSON.Get("devDependencies"); devDeps != nil {
		if dep := devDeps.Get(packageName); dep != nil {
			if version, err := dep.String(); err == nil {
				return cleanVersion(version)
			}
		}
	}

	return ""
}

// cleanVersion removes version prefixes like ^, ~, >=
func cleanVersion(version string) string {
	if len(version) == 0 {
		return version
	}

	// Remove common prefixes
	for _, prefix := range []string{"^", "~", ">=", ">", "<=", "<", "="} {
		if len(version) > len(prefix) && version[:len(prefix)] == prefix {
			return version[len(prefix):]
		}
	}

	return version
}

// hasAnyFile checks if any of the specified files exist in the build path
func hasAnyFile(buildPath string, files []string) bool {
	for _, file := range files {
		filePath := path.Join(buildPath, file)
		if ok, _ := util.FileExists(filePath); ok {
			return true
		}
	}
	return false
}

// GetDisplayName returns the display name for a framework
func GetDisplayName(frameworkName string) string {
	for _, detector := range frameworkDetectors {
		if detector.name == frameworkName {
			return detector.displayName
		}
	}
	return frameworkName
}

// GetSupportedFrameworks returns list of all supported frameworks
func GetSupportedFrameworks() []Framework {
	frameworks := make([]Framework, 0, len(frameworkDetectors))
	for _, detector := range frameworkDetectors {
		frameworks = append(frameworks, Framework{
			Name:        detector.name,
			DisplayName: detector.displayName,
			RuntimeType: detector.runtimeType,
			OutputDir:   detector.outputDir,
			BuildCmd:    detector.buildCmd,
			StartCmd:    detector.startCmd,
		})
	}
	return frameworks
}
