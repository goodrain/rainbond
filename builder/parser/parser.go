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

package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/builder/parser/code"
)

//Port 端口
type Port struct {
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

//Volume 存储地址
type Volume struct {
	VolumePath string `json:"volume_path"`
	VolumeType string `json:"volume_type"`
}

//Env env desc
type Env struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

//ParseError 错误信息
type ParseError struct {
	ErrorType   ParseErrorType `json:"error_type"`
	ErrorInfo   string         `json:"error_info"`
	SolveAdvice string         `json:"solve_advice"`
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

//SolveAdvice 构建a标签建议
func SolveAdvice(actionType, message string) string {
	return fmt.Sprintf("<a action_type=\"%s\">%s</a>", actionType, message)
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
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

func (i Image) String() string {
	return fmt.Sprintf("%s:%s", i.Name, i.Tag)
}

//Parser 解析器
type Parser interface {
	Parse() ParseErrorList
	GetServiceInfo() []ServiceInfo
	GetImage() Image
}

//Lang 语言类型
type Lang string

//ServiceInfo 智能获取的应用信息
type ServiceInfo struct {
	Ports             []Port    `json:"ports"`
	Envs              []Env     `json:"envs"`
	Volumes           []Volume  `json:"volumes"`
	Image             Image     `json:"image"`
	Args              []string  `json:"args"`
	DependServices    []string  `json:"depends,omitempty"`
	ServiceDeployType string    `json:"deploy_type,omitempty"`
	Branchs           []string  `json:"branchs,omitempty"`
	Memory            int       `json:"memory"`
	Lang              code.Lang `json:"language"`
	Runtime           bool      `json:"runtime"`
	Dependencies      bool      `json:"dependencies"`
	Procfile          bool      `json:"procfile"`
	ImageAlias        string    `json:"image_alias"`
}

//GetServiceInfo GetServiceInfo
type GetServiceInfo struct {
	UUID   string `json:"uuid"`
	Source string `json:"source"`
}

//GetPortProtocol 获取端口协议
func GetPortProtocol(port int) string {
	if port == 80 {
		return "http"
	}
	if port == 8080 {
		return "http"
	}
	if port == 22 {
		return "tcp"
	}
	if port == 3306 {
		return "mysql"
	}
	if port == 443 {
		return "https"
	}
	if port == 3128 {
		return "http"
	}
	if port == 1080 {
		return "udp"
	}
	if port == 6379 {
		return "tcp"
	}
	if port > 1 && port < 5000 {
		return "tcp"
	}
	return "http"
}

//readmemory
//10m 10
//10g 10*1024
//10k 128
//10b 128
func readmemory(s string) int {
	if strings.HasSuffix(s, "m") {
		s, err := strconv.Atoi(s[0 : len(s)-1])
		if err != nil {
			return 128
		}
		return s
	}
	if strings.HasSuffix(s, "g") {
		s, err := strconv.Atoi(s[0 : len(s)-1])
		if err != nil {
			return 128
		}
		return s * 1024
	}
	return 128
}

func parseImageName(s string) Image {
	index := strings.LastIndex(s, ":")
	if index > -1 {
		return Image{
			Name: s[0:index],
			Tag:  s[index+1:],
		}
	}
	return Image{
		Name: s,
		Tag:  "latest",
	}
}
