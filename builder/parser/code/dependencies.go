// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

	"github.com/goodrain/rainbond/util"
)

//CheckDependencies check dependencies with lang
func CheckDependencies(buildPath string, lang Lang) bool {
	switch lang {
	case PHP:
		if ok, _ := util.FileExists(path.Join(buildPath, "composer.json")); ok {
			return true
		}
		return false
	case Python:
		if ok, _ := util.FileExists(path.Join(buildPath, "requirements.txt")); ok {
			return true
		}
		return false

	case Ruby:
		return true
	case JavaMaven:
		if ok, _ := util.FileExists(path.Join(buildPath, "pom.xml")); ok {
			return true
		}
		return false

	case JaveWar, JavaJar:
		return true
	case Nodejs:
		if ok, _ := util.FileExists(path.Join(buildPath, "package.json ")); ok {
			return true
		}
		return false
	default:
		return true
	}
}
