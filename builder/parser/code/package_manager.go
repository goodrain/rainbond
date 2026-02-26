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
	"path"
	"strings"

	"github.com/goodrain/rainbond/util"
)

// PackageManager represents the type of package manager
type PackageManager string

const (
	// PackageManagerNPM npm package manager
	PackageManagerNPM PackageManager = "npm"
	// PackageManagerYarn yarn package manager
	PackageManagerYarn PackageManager = "yarn"
	// PackageManagerPNPM pnpm package manager
	PackageManagerPNPM PackageManager = "pnpm"
)

// PackageManagerInfo contains detected package manager information
type PackageManagerInfo struct {
	Manager  PackageManager
	LockFile string
	Version  string // parsed from packageManager field in package.json
}

// DetectPackageManager detects the package manager used in a Node.js project
// Detection priority: pnpm-lock.yaml > yarn.lock > package-lock.json
func DetectPackageManager(buildPath string) PackageManagerInfo {
	// 1. First check package.json's packageManager field (Corepack)
	// Example: "packageManager": "pnpm@8.15.0"
	if pm := readPackageManagerField(buildPath); pm != nil {
		return *pm
	}

	// 2. Detect by lock file (by priority)
	if ok, _ := util.FileExists(path.Join(buildPath, "pnpm-lock.yaml")); ok {
		return PackageManagerInfo{Manager: PackageManagerPNPM, LockFile: "pnpm-lock.yaml"}
	}
	if ok, _ := util.FileExists(path.Join(buildPath, "yarn.lock")); ok {
		return PackageManagerInfo{Manager: PackageManagerYarn, LockFile: "yarn.lock"}
	}
	if ok, _ := util.FileExists(path.Join(buildPath, "package-lock.json")); ok {
		return PackageManagerInfo{Manager: PackageManagerNPM, LockFile: "package-lock.json"}
	}

	// 3. Default to npm
	return PackageManagerInfo{Manager: PackageManagerNPM}
}

// readPackageManagerField reads the packageManager field from package.json
// This field is used by Corepack to determine the package manager
// Format: "packageManager": "pnpm@8.15.0" or "yarn@4.0.0"
func readPackageManagerField(buildPath string) *PackageManagerInfo {
	pkgJSON := readPackageJSON(buildPath)
	if pkgJSON == nil {
		return nil
	}

	pmField := pkgJSON.Get("packageManager")
	if pmField == nil {
		return nil
	}

	pmValue, err := pmField.String()
	if err != nil || pmValue == "" {
		return nil
	}

	// Parse format: "pnpm@8.15.0" or "yarn@4.0.0" or "npm@10.0.0"
	return parsePackageManagerField(pmValue)
}

// parsePackageManagerField parses the packageManager field value
// Format: "manager@version" e.g., "pnpm@8.15.0"
func parsePackageManagerField(value string) *PackageManagerInfo {
	parts := strings.SplitN(value, "@", 2)
	if len(parts) == 0 {
		return nil
	}

	managerName := strings.ToLower(strings.TrimSpace(parts[0]))
	version := ""
	if len(parts) > 1 {
		version = strings.TrimSpace(parts[1])
	}

	var manager PackageManager
	var lockFile string

	switch managerName {
	case "pnpm":
		manager = PackageManagerPNPM
		lockFile = "pnpm-lock.yaml"
	case "yarn":
		manager = PackageManagerYarn
		lockFile = "yarn.lock"
	case "npm":
		manager = PackageManagerNPM
		lockFile = "package-lock.json"
	default:
		return nil
	}

	return &PackageManagerInfo{
		Manager:  manager,
		LockFile: lockFile,
		Version:  version,
	}
}

// GetInstallCommand returns the install command for the package manager
func (pm PackageManagerInfo) GetInstallCommand() string {
	switch pm.Manager {
	case PackageManagerPNPM:
		return "pnpm install --frozen-lockfile"
	case PackageManagerYarn:
		return "yarn install --frozen-lockfile"
	case PackageManagerNPM:
		return "npm ci"
	default:
		return "npm install"
	}
}

// GetBuildCommand returns the build command for the package manager
func (pm PackageManagerInfo) GetBuildCommand() string {
	switch pm.Manager {
	case PackageManagerPNPM:
		return "pnpm run build"
	case PackageManagerYarn:
		return "yarn build"
	case PackageManagerNPM:
		return "npm run build"
	default:
		return "npm run build"
	}
}

// GetStartCommand returns the start command for the package manager
func (pm PackageManagerInfo) GetStartCommand() string {
	switch pm.Manager {
	case PackageManagerPNPM:
		return "pnpm start"
	case PackageManagerYarn:
		return "yarn start"
	case PackageManagerNPM:
		return "npm start"
	default:
		return "npm start"
	}
}

// String returns the string representation of the package manager
func (pm PackageManager) String() string {
	return string(pm)
}
