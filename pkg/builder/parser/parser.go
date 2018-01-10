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

package parser

import (
	"fmt"
)

//Port 端口
type Port struct {
	ContainerPort int
	Protocol      string
}

//Volume 存储地址
type Volume struct {
	VolumePath string
	VolumeType string
}

//Env env desc
type Env struct {
	Name  string
	Value string
}

//ParseError 错误信息
type ParseError struct {
	ErrorType   ParseErrorType
	ErrorInfo   string
	SolveAdvice string
}

//ParseErrorList 错误列表
type ParseErrorList []ParseError

//ParseErrorType 错误类型
type ParseErrorType string

//FatalError 致命错误
var FatalError ParseErrorType = "FatalError"

//NegligibleError 可忽略错误
var NegligibleError ParseErrorType = "NegligibleError"

//Errorf error create
func Errorf(errtype ParseErrorType, format string, a ...interface{}) ParseError {
	parseError := ParseError{
		ErrorType: errtype,
		ErrorInfo: fmt.Sprintf(format, a...),
	}
	return parseError
}

//ErrorAndSolve error create
func ErrorAndSolve(errtype ParseErrorType, errorInfo, SolveAdvice string) ParseError {
	parseError := ParseError{
		ErrorType:   errtype,
		ErrorInfo:   errorInfo,
		SolveAdvice: SolveAdvice,
	}
	return parseError
}

func (p ParseError) Error() string {
	return fmt.Sprintf("Type:%s Message:%s", p.ErrorType, p.ErrorInfo)
}
func (ps ParseErrorList) Error() string {
	var re string
	for _, p := range ps {
		re += fmt.Sprintf("Type:%s Message:%s\n", p.ErrorType, p.ErrorInfo)
	}
	return re
}

//IsFatalError 是否具有致命错误
func (ps ParseErrorList) IsFatalError() bool {
	for _, p := range ps {
		if p.ErrorType == FatalError {
			return true
		}
	}
	return false
}

//Image 镜像
type Image struct {
	Name string
	Tag  string
}

func (i Image) String() string {
	return fmt.Sprintf("%s:%s", i.Name, i.Tag)
}

//Parser 解析器
type Parser interface {
	Parse() ParseErrorList
	//获取分支列表
	GetBranchs() []string
	//获取端口列表
	GetPorts() []Port
	//获取存储列表
	GetVolumes() []Volume
	//获取源是否合法
	GetValid() bool
	//获取环境变量
	GetEnvs() []Env
	//获取镜像名
	GetImage() Image
	//获取启动参数
	GetArgs() []string
	//获取内存
	GetMemory() int
}
