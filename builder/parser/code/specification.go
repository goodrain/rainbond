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
	"io/ioutil"
	"path"
	"strings"

	"github.com/goodrain/rainbond/util"
)

//Specification 规范
type Specification struct {
	//是否符合规范
	Conform bool
	//不符合项目 解决方式
	Noconform map[string]string
	//建议规范项目 处理方式
	Advice map[string]string
}

//各类型语言规范
var specification map[Lang]func(buildPath string) Specification

func init() {
	specification = make(map[Lang]func(buildPath string) Specification)
	specification[JavaJar] = javaJarCheck
	specification[JavaMaven] = javaMavenCheck
}

//CheckCodeSpecification 检查语言规范
func CheckCodeSpecification(buildPath string, lang Lang) Specification {
	if check, ok := specification[lang]; ok {
		return check(buildPath)
	}
	return common()
}

//必须定义Procfile文件
//Procfile文件中定义的jar包必须存在
func javaJarCheck(buildPath string) Specification {
	procfile, spec := checkProcfile(buildPath)
	if spec != nil {
		return *spec
	}
	if !procfile {
		return Specification{
			Conform:   false,
			Noconform: map[string]string{"识别为JavaJar语言,Procfile文件未定义": "主目录定义Procfile文件指定jar包启动方式，参考格式:\n web: java $JAVA_OPTS -jar demo.jar"},
		}
	}
	return common()
}

// 查找 pom.xml 文件中是否包含 org.springframework.boot
// 不包含（强制）
// pom.xml中必须引入 webapp-runner.jar
// packaging类型必须为war
// 包含
// 建议写Procfile（不写的话平台默认设置）
func javaMavenCheck(buildPath string) Specification {
	procfile, spec := checkProcfile(buildPath)
	if spec != nil {
		return *spec
	}
	if ok, _ := util.FileExists(path.Join(buildPath, "pom.xml")); !ok {
		return Specification{
			Conform:   false,
			Noconform: map[string]string{"识别为JavaMaven语言，工作目录未发现pom.xml文件": "定义pom.xml文件"},
		}
	}
	ok := util.SearchFileBody(path.Join(buildPath, "pom.xml"), "<modules>")
	if ok {
		return common()
	}
	//判断pom.xml中是否包含 org.springframework.boot定义
	ok = util.SearchFileBody(path.Join(buildPath, "pom.xml"), "org.springframework.boot")
	if !ok {
		//默认只能打包成war包
		war := util.SearchFileBody(path.Join(buildPath, "pom.xml"), "<packaging>war</packaging>")
		if !war && !procfile {
			//如果定义成jar包，必须定义Procfile
			return Specification{
				Conform:   false,
				Noconform: map[string]string{"识别为JavaMaven语言，非SpringBoot项目默认只支持War打包": "参看官方JaveMaven项目代码配置"},
			}
		}
		//TODO: 检查procfile定义是否正确
	}
	return common()
}

//checkProcfile 检查Procfile文件
func checkProcfile(buildPath string) (bool, *Specification) {
	if ok, _ := util.FileExists(path.Join(buildPath, "Procfile")); !ok {
		return false, nil
	}
	procfile, err := ioutil.ReadFile(path.Join(buildPath, "Procfile"))
	if err != nil {
		return false, nil
	}
	infos := strings.Split(strings.TrimRight(string(procfile), " "), " ")
	if len(infos) < 2 {
		return true, &Specification{
			Conform:   false,
			Noconform: map[string]string{"Procfile文件不符合规范": "参考格式\n web: 启动命令"},
		}
	}
	if infos[0] != "web:" {
		return true, &Specification{
			Conform:   false,
			Noconform: map[string]string{"Procfile文件规范目前只支持 web: 开头": "参考格式\n web: 启动命令"},
		}
	}
	return true, nil
}
func common() Specification {
	return Specification{
		Conform: true,
	}
}
