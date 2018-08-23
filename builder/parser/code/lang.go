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
	"path"

	"github.com/goodrain/rainbond/util"
)

func init() {
	checkFuncList = append(checkFuncList, dockerfile)
	checkFuncList = append(checkFuncList, javaJar)
	checkFuncList = append(checkFuncList, javaWar)
	checkFuncList = append(checkFuncList, javaMaven)
	checkFuncList = append(checkFuncList, php)
	checkFuncList = append(checkFuncList, python)
	checkFuncList = append(checkFuncList, nodejs)
	checkFuncList = append(checkFuncList, ruby)
	checkFuncList = append(checkFuncList, static)
	checkFuncList = append(checkFuncList, clojure)
	checkFuncList = append(checkFuncList, golang)
	checkFuncList = append(checkFuncList, gradle)
	checkFuncList = append(checkFuncList, grails)
	checkFuncList = append(checkFuncList, scala)
	checkFuncList = append(checkFuncList, netcore)
}

//ErrCodeNotExist 代码为空错误
var ErrCodeNotExist = fmt.Errorf("code is not exist")

//ErrCodeDirNotExist 代码目录不存在
var ErrCodeDirNotExist = fmt.Errorf("code dir is not exist")

//ErrCodeUnableIdentify 代码无法识别语言
var ErrCodeUnableIdentify = fmt.Errorf("code lang unable to identify")

//ErrRainbondFileNotFound rainbond file not found
var ErrRainbondFileNotFound = fmt.Errorf("rainbond file not found")

//Lang 语言类型
type Lang string

//String return lang string
func (l Lang) String() string {
	return string(l)
}

//NO 空语言类型
var NO Lang = "no"

//Dockerfile Lang
var Dockerfile Lang = "dockerfile"

//Docker Lang
var Docker Lang = "docker"

//Python Lang
var Python Lang = "Python"

//Ruby Lang
var Ruby Lang = "Ruby"

//PHP Lang
var PHP Lang = "PHP"

//JavaMaven Lang
var JavaMaven Lang = "Java-maven"

//JaveWar Lang
var JaveWar Lang = "Java-war"

//JavaJar Lang
var JavaJar Lang = "Java-jar"

//Nodejs Lang
var Nodejs Lang = "Node.js"

//Static Lang
var Static Lang = "static"

//Clojure Lang
var Clojure Lang = "Clojure"

//Golang Lang
var Golang Lang = "Go"

//Gradle Lang
var Gradle Lang = "Gradle"

//Grails Lang
var Grails Lang = "Grails"

//NetCore Lang
var NetCore Lang = ".NetCore"

//GetLangType check code lang
func GetLangType(homepath string) (Lang, error) {
	if ok, _ := util.FileExists(homepath); !ok {
		return NO, ErrCodeDirNotExist
	}
	//判断是否有代码
	if ok := util.IsHaveFile(homepath); !ok {
		return NO, ErrCodeNotExist
	}
	//获取确定的语言
	for _, check := range checkFuncList {
		if lang := check(homepath); lang != NO {
			return lang, nil
		}
	}
	//获取可能的语言
	//无法识别
	return NO, ErrCodeUnableIdentify
}

type langTypeFunc func(homepath string) Lang

var checkFuncList []langTypeFunc

func dockerfile(homepath string) Lang {
	if ok, _ := util.FileExists(path.Join(homepath, "Dockerfile")); !ok {
		return NO
	}
	return Dockerfile
}
func python(homepath string) Lang {
	if ok, _ := util.FileExists(path.Join(homepath, "requirements.txt")); ok {
		return Python
	}
	if ok, _ := util.FileExists(path.Join(homepath, "setup.py")); ok {
		return Python
	}
	return NO
}
func ruby(homepath string) Lang {
	if ok, _ := util.FileExists(path.Join(homepath, "Gemfile")); ok {
		return Ruby
	}
	return NO
}
func php(homepath string) Lang {
	if ok, _ := util.FileExists(path.Join(homepath, "composer.json")); ok {
		return PHP
	}
	if ok := util.SearchFile(homepath, "index.php", 2); ok {
		return PHP
	}
	return NO
}
func javaMaven(homepath string) Lang {
	if ok, _ := util.FileExists(path.Join(homepath, "pom.xml")); ok {
		return JavaMaven
	}
	return NO
}
func javaWar(homepath string) Lang {
	if ok := util.FileExistsWithSuffix(homepath, ".war"); ok {
		return JaveWar
	}
	return NO
}

//javaJar Procfile必须定义
func javaJar(homepath string) Lang {
	if ok := util.FileExistsWithSuffix(homepath, ".jar"); ok {
		return JavaJar
	}
	return NO
}
func nodejs(homepath string) Lang {
	if ok, _ := util.FileExists(path.Join(homepath, "package.json")); ok {
		return Nodejs
	}
	return NO
}
func static(homepath string) Lang {
	if ok, _ := util.FileExists(path.Join(homepath, "index.html")); ok {
		return Static
	}
	if ok, _ := util.FileExists(path.Join(homepath, "index.htm")); ok {
		return Static
	}
	return NO
}
func clojure(homepath string) Lang {
	if ok, _ := util.FileExists(path.Join(homepath, "project.clj")); ok {
		return Clojure
	}
	return NO
}
func golang(homepath string) Lang {
	if ok, _ := util.FileExists(path.Join(homepath, "Godeps", "Godeps.json")); ok {
		return Golang
	}
	if ok, _ := util.FileExists(path.Join(homepath, "vendor", "Govendor.json")); ok {
		return Golang
	}
	if ok := util.FileExistsWithSuffix(path.Join(homepath, "src"), ".go"); ok {
		return Golang
	}
	return NO
}
func gradle(homepath string) Lang {
	if ok, _ := util.FileExists(path.Join(homepath, "build.gradle")); ok {
		return Gradle
	}
	return NO
}
func grails(homepath string) Lang {
	if ok, _ := util.FileExists(path.Join(homepath, "grails-app")); ok {
		return Grails
	}
	return NO
}

//netcore
func netcore(homepath string) Lang {
	if ok := util.FileExistsWithSuffix(homepath, ".sln"); ok {
		return NetCore
	}
	if ok := util.FileExistsWithSuffix(homepath, ".csproj"); ok {
		return NetCore
	}
	return NO
}

//暂时不支持
func scala(homepath string) Lang {
	return NO
}
