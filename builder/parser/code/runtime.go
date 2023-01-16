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
	"io/ioutil"
	"path"
	"strings"

	simplejson "github.com/bitly/go-simplejson"

	"github.com/goodrain/rainbond/util"
)

//ErrRuntimeNotSupport runtime not support
var ErrRuntimeNotSupport = fmt.Errorf("runtime version not support")

//CheckRuntime CheckRuntime
func CheckRuntime(buildPath string, lang Lang) (map[string]string, error) {
	switch lang {
	case PHP:
		return readPHPRuntimeInfo(buildPath)
	case Python:
		return readPythonRuntimeInfo(buildPath)
	case JavaMaven, JaveWar, JavaJar:
		return readJavaRuntimeInfo(buildPath)
	case Nodejs:
		return readNodeRuntimeInfo(buildPath)
	case NodeJSStatic:
		runtime, err := readNodeRuntimeInfo(buildPath)
		if err != nil {
			return nil, err
		}
		runtime["RUNTIMES_SERVER"] = "nginx"
		return runtime, nil
	case Static:
		return map[string]string{"RUNTIMES_SERVER": "nginx"}, nil
	default:
		return nil, nil
	}
}

func readPHPRuntimeInfo(buildPath string) (map[string]string, error) {
	var phpRuntimeInfo = make(map[string]string, 1)
	if ok, _ := util.FileExists(path.Join(buildPath, "composer.json")); !ok {
		return phpRuntimeInfo, nil
	}
	body, err := ioutil.ReadFile(path.Join(buildPath, "composer.json"))
	if err != nil {
		return phpRuntimeInfo, nil
	}
	json, err := simplejson.NewJson(body)
	if err != nil {
		return phpRuntimeInfo, nil
	}
	getPhpNewVersion := func(v string) string {
		version := v
		switch v {
		case "5.5":
			version = "5.5.38"
		case "5.6":
			version = "5.6.35"
		case "7.0":
			version = "7.0.29"
		case "7.1":
			version = "7.1.33"
		case "7.2":
			version = "7.2.26"
		case "7.3":
			version = "7.3.13"
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
					if !util.StringArrayContains([]string{"5.5", "5.6", "7.0", "7.1", "7.3"}, version[2:3]) {
						return nil, ErrRuntimeNotSupport
					}
					version = getPhpNewVersion(version[2:3])
				}
				if version[0] == '~' {
					if !util.StringArrayContains([]string{"5.5", "5.6", "7.0", "7.1", "7.3"}, version[1:3]) {
						return nil, ErrRuntimeNotSupport
					}
					version = getPhpNewVersion(version[1:3])
				} else {
					if !util.StringArrayContains([]string{"5.5", "5.6", "7.0", "7.1", "7.3"}, version[0:3]) {
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
	body, err := ioutil.ReadFile(path.Join(buildPath, "runtime.txt"))
	if err != nil {
		return runtimeInfo, nil
	}
	runtimeInfo["RUNTIMES"] = string(body)
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
		runtimeInfo["RUNTIMES"] = runtime
	}
	return runtimeInfo, nil
}

func readNodeRuntimeInfo(buildPath string) (map[string]string, error) {
	var runtimeInfo = make(map[string]string, 1)
	if ok, _ := util.FileExists(path.Join(buildPath, "package.json")); !ok {
		return runtimeInfo, nil
	}
	body, err := ioutil.ReadFile(path.Join(buildPath, "package.json"))
	if err != nil {
		return runtimeInfo, nil
	}
	json, err := simplejson.NewJson(body)
	if err != nil {
		return runtimeInfo, nil
	}
	if json.Get("engines") != nil {
		if v := json.Get("engines").Get("node"); v != nil {
			nodeVersion, _ := v.String()
			// The latest version is used by default. (11.1.0 is latest version in ui)
			if strings.HasPrefix(nodeVersion, ">") || strings.HasPrefix(nodeVersion, "*") || strings.HasPrefix(nodeVersion, "^") {
				nodeVersion = "11.1.0"
			}
			runtimeInfo["RUNTIMES"] = nodeVersion
		}
	}
	// default npm
	runtimeInfo["PACKAGE_TOOL"] = "npm"
	return runtimeInfo, nil
}
