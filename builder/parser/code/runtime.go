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
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/goodrain/rainbond/db"
	"github.com/jinzhu/gorm"

	simplejson "github.com/bitly/go-simplejson"

	"github.com/goodrain/rainbond/util"
)

// ErrRuntimeNotSupport runtime not support
var ErrRuntimeNotSupport = fmt.Errorf("runtime version not support")

// CheckRuntime CheckRuntime
func CheckRuntime(buildPath string, lang Lang) (map[string]string, error) {
	// Handle combined language types (e.g., "Node.js,static")
	langStr := string(lang)

	// Check for Node.js related languages first
	// Note: NodeJSStatic is deprecated, all Node.js projects use Nodejs type now
	if strings.Contains(langStr, string(Nodejs)) {
		return readNodeRuntimeInfo(buildPath)
	}

	switch lang {
	case PHP:
		return readPHPRuntimeInfo(buildPath)
	case Python:
		return readPythonRuntimeInfo(buildPath)
	case JavaMaven, JaveWar, JavaJar:
		return readJavaRuntimeInfo(buildPath)
	case Nodejs:
		return readNodeRuntimeInfo(buildPath)
	case Static:
		return map[string]string{}, nil
	default:
		return nil, nil
	}
}

func readPHPRuntimeInfo(buildPath string) (map[string]string, error) {
	var phpRuntimeInfo = make(map[string]string, 1)
	if ok, _ := util.FileExists(path.Join(buildPath, "composer.json")); !ok {
		return phpRuntimeInfo, nil
	}
	body, err := os.ReadFile(path.Join(buildPath, "composer.json"))
	if err != nil {
		return phpRuntimeInfo, nil
	}
	json, err := simplejson.NewJson(body)
	if err != nil {
		return phpRuntimeInfo, nil
	}
	getPhpNewVersion := func(v string) string {
		version := v
		vv, err := db.GetManager().LongVersionDao().GetVersionByLanguageAndVersion("php", version)
		if (err != nil && err == gorm.ErrRecordNotFound) || !vv.Show {
			ver, err := db.GetManager().LongVersionDao().GetDefaultVersionByLanguageAndVersion("php")
			if err != nil {
				return version
			}
			return ver.Version
		}
		return version
	}
	if json.Get("require") != nil {
		if phpVersion := json.Get("require").Get("php"); phpVersion != nil {
			version, _ := phpVersion.String()
			if version != "" {
				if len(version) < 4 || (version[0:2] == ">=" && len(version) < 5) {
					return nil, ErrRuntimeNotSupport
				}
				if version[0:2] == ">=" {
					if !util.StringArrayContains([]string{"7.1", "8.1", "8.2"}, version[2:3]) {
						return nil, ErrRuntimeNotSupport
					}
					version = getPhpNewVersion(version[2:3])
				}
				if version[0] == '~' {
					if !util.StringArrayContains([]string{"7.1", "8.1", "8.2"}, version[1:3]) {
						return nil, ErrRuntimeNotSupport
					}
					version = getPhpNewVersion(version[1:3])
				} else {
					if !util.StringArrayContains([]string{"7.1", "8.1", "8.2"}, version[0:3]) {
						return nil, ErrRuntimeNotSupport
					}
					version = getPhpNewVersion(version[0:3])
				}
				phpRuntimeInfo["RUNTIMES"] = version
			}
		}
		if hhvmVersion := json.Get("require").Get("hhvm"); hhvmVersion != nil {
			phpRuntimeInfo["RUNTIMES_HHVM"], _ = hhvmVersion.String()
		}
	}
	return phpRuntimeInfo, nil
}

func readPythonRuntimeInfo(buildPath string) (map[string]string, error) {
	var runtimeInfo = make(map[string]string, 1)
	if ok, _ := util.FileExists(path.Join(buildPath, "runtime.txt")); !ok {
		return runtimeInfo, nil
	}
	body, err := os.ReadFile(path.Join(buildPath, "runtime.txt"))
	if err != nil {
		return runtimeInfo, nil
	}
	version := string(body)
	v, err := db.GetManager().LongVersionDao().GetVersionByLanguageAndVersion("python", version)
	if (err != nil && err == gorm.ErrRecordNotFound) || !v.Show {
		ver, err := db.GetManager().LongVersionDao().GetDefaultVersionByLanguageAndVersion("python")
		if err != nil {
			return runtimeInfo, nil
		}
		version = ver.Version
	}
	runtimeInfo["RUNTIMES"] = version
	return runtimeInfo, nil
}

func readJavaRuntimeInfo(buildPath string) (map[string]string, error) {
	var runtimeInfo = make(map[string]string, 1)
	ok, err := util.FileExists(path.Join(buildPath, "system.properties"))
	if !ok || err != nil {
		return runtimeInfo, nil
	}
	cmd := fmt.Sprintf(`grep -i "java.runtime.version" %s | grep  -E -o "[0-9]+(.[0-9]+)?(.[0-9]+)?"`, path.Join(buildPath, "system.properties"))
	runtime, err := util.CmdExec(cmd)
	if err != nil {
		return runtimeInfo, nil
	}
	if runtime != "" {
		vv, err := db.GetManager().LongVersionDao().GetVersionByLanguageAndVersion("openJDK", runtime)
		if (err != nil && err == gorm.ErrRecordNotFound) && !vv.Show {
			ver, err := db.GetManager().LongVersionDao().GetDefaultVersionByLanguageAndVersion("openJDK")
			if err != nil {
				return runtimeInfo, nil
			}
			runtime = ver.Version
		}
		runtimeInfo["RUNTIMES"] = runtime
	}
	return runtimeInfo, nil
}

func readNodeRuntimeInfo(buildPath string) (map[string]string, error) {
	var runtimeInfo = make(map[string]string, 1)
	if ok, _ := util.FileExists(path.Join(buildPath, "package.json")); !ok {
		return runtimeInfo, nil
	}
	body, err := os.ReadFile(path.Join(buildPath, "package.json"))
	if err != nil {
		return runtimeInfo, nil
	}
	json, err := simplejson.NewJson(body)
	if err != nil {
		return runtimeInfo, nil
	}

	// Parse Node.js version using enhanced version resolver
	if json.Get("engines") != nil {
		if v := json.Get("engines").Get("node"); v != nil {
			nodeVersion, _ := v.String()
			if nodeVersion != "" {
				// Use the new version resolver
				versionInfo := ResolveNodeVersion(nodeVersion)

				// Try to get version from database first (for backward compatibility)
				if strings.HasPrefix(nodeVersion, ">") || strings.HasPrefix(nodeVersion, "*") || strings.HasPrefix(nodeVersion, "^") {
					vv, err := db.GetManager().LongVersionDao().GetVersionByLanguageAndVersion("node", nodeVersion)
					if (err != nil && err == gorm.ErrRecordNotFound) || !vv.Show {
						// Database doesn't have this version, use resolved version
						nodeVersion = versionInfo.Resolved
					}
				} else {
					// For non-range versions, use the resolved version
					nodeVersion = versionInfo.Resolved
				}

				runtimeInfo["RUNTIMES"] = nodeVersion
				runtimeInfo["NODE_VERSION_ORIGINAL"] = versionInfo.Original
				runtimeInfo["NODE_VERSION_SOURCE"] = versionInfo.Source
			}
		}
	}

	// If no version specified, use default
	if runtimeInfo["RUNTIMES"] == "" {
		runtimeInfo["RUNTIMES"] = DefaultNodeVersion
		runtimeInfo["NODE_VERSION_SOURCE"] = "default"
	}

	// Detect package manager (replaces hardcoded npm)
	pmInfo := DetectPackageManager(buildPath)
	runtimeInfo["PACKAGE_TOOL"] = string(pmInfo.Manager)
	if pmInfo.LockFile != "" {
		runtimeInfo["PACKAGE_LOCK_FILE"] = pmInfo.LockFile
	}
	if pmInfo.Version != "" {
		runtimeInfo["PACKAGE_MANAGER_VERSION"] = pmInfo.Version
	}

	// Detect framework
	if framework := DetectFramework(buildPath); framework != nil {
		runtimeInfo["FRAMEWORK"] = framework.Name
		runtimeInfo["FRAMEWORK_DISPLAY_NAME"] = framework.DisplayName
		if framework.Version != "" {
			runtimeInfo["FRAMEWORK_VERSION"] = framework.Version
		}
		runtimeInfo["RUNTIME_TYPE"] = framework.RuntimeType
		if framework.OutputDir != "" {
			runtimeInfo["OUTPUT_DIR"] = framework.OutputDir
		}
		if framework.BuildCmd != "" {
			runtimeInfo["BUILD_CMD"] = framework.BuildCmd
		}
		if framework.StartCmd != "" {
			runtimeInfo["START_CMD"] = framework.StartCmd
		}
	}

	// Detect config files
	configFiles := DetectConfigFiles(buildPath)
	runtimeInfo["HAS_NPMRC"] = boolToString(configFiles.HasNpmrc)
	runtimeInfo["HAS_YARNRC"] = boolToString(configFiles.HasYarnrc)

	return runtimeInfo, nil
}

// boolToString converts a boolean to "true" or "false" string
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
