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
	"strings"

	dbmodel "github.com/goodrain/rainbond/db/model"

	"github.com/docker/distribution/reference"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/builder/parser/discovery"
	"github.com/goodrain/rainbond/builder/parser/types"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"
)

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
	name   reference.Named
	source string
	Name   string `json:"name"`
	Tag    string `json:"tag"`
}

//String -
func (i Image) String() string {
	if i.name == nil {
		return ""
	}
	return i.name.String()
}

//Source return the name before resolution
func (i Image) Source() string {
	return i.source
}

//GetTag get tag
func (i Image) GetTag() string {
	return i.Tag
}

//GetRepostory get repostory
func (i Image) GetRepostory() string {
	if i.name == nil {
		return ""
	}
	return reference.Path(i.name)
}

//GetDomain get image registry domain
func (i Image) GetDomain() string {
	if i.name == nil {
		return ""
	}
	domain := reference.Domain(i.name)
	if domain == "docker.io" {
		domain = "registry-1.docker.io"
	}
	return domain
}

//IsOfficial is official image
func (i Image) IsOfficial() bool {
	domain := reference.Domain(i.name)
	if domain == "docker.io" {
		return true
	}
	return false
}

//GetSimpleName get image name without tag and organizations
func (i Image) GetSimpleName() string {
	if strings.Contains(i.GetRepostory(), "/") {
		return strings.Split(i.GetRepostory(), "/")[1]
	}
	return i.GetRepostory()
}

//GetNamespace get namespace
func (i Image) GetNamespace() string {
	if strings.Contains(i.GetRepostory(), "/") {
		return strings.Split(i.GetRepostory(), "/")[0]
	}
	return ""
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
	ID             string         `json:"id,omitempty"`
	Ports          []types.Port   `json:"ports,omitempty"`
	Envs           []types.Env    `json:"envs,omitempty"`
	Volumes        []types.Volume `json:"volumes,omitempty"`
	Image          Image          `json:"image,omitempty"`
	Args           []string       `json:"args,omitempty"`
	DependServices []string       `json:"depends,omitempty"`
	ServiceType    string         `json:"service_type,omitempty"`
	Branchs        []string       `json:"branchs,omitempty"`
	Memory         int            `json:"memory,omitempty"`
	Lang           code.Lang      `json:"language,omitempty"`
	ImageAlias     string         `json:"image_alias,omitempty"`
	//For third party services
	Endpoints []*discovery.Endpoint `json:"endpoints,omitempty"`
	//os type,default linux
	OS        string `json:"os"`
	Name      string `json:"name,omitempty"`  // module name
	Cname     string `json:"cname,omitempty"` // service cname
	Packaging string `json:"packaging,omitempty"`
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

var dbImageKey = []string{
	"mysql", "mariadb", "mongo", "redis", "tidb",
	"zookeeper", "kafka", "mysqldb", "mongodb",
	"memcached", "cockroachdb", "cockroach", "etcd",
	"postgres", "postgresql", "elasticsearch", "consul",
	"percona", "mysql-server", "mysql-cluster",
}

//DetermineDeployType Determine the deployment type
// if image like db image,return stateful type
func DetermineDeployType(imageName Image) string {
	for _, key := range dbImageKey {
		if strings.ToLower(imageName.GetSimpleName()) == key {
			return dbmodel.ServiceTypeStateSingleton.String()
		}
	}
	return dbmodel.ServiceTypeStatelessMultiple.String()
}

//readmemory
//10m 10
//10g 10*1024
//10k 128
//10b 128
func readmemory(s string) int {
	def := 512
	s = strings.ToLower(s)
	// <binarySI>        ::= Ki | Mi | Gi | Ti | Pi | Ei
	isValid := false
	validUnits := map[string]string{
		"gi": "Gi", "mi": "Mi", "ki": "Ki",
	}
	for k, v := range validUnits {
		if strings.Contains(s, k) {
			isValid = true
			s = strings.Replace(s, k, v, 1)
			break
		}
	}
	if !isValid {
		validUnits := map[string]string{
			"g": "Gi", "m": "Mi", "k": "Ki",
		}
		for k, v := range validUnits {
			if strings.Contains(s, k) {
				isValid = true
				s = strings.Replace(s, k, v, 1)
				break
			}
		}
	}
	if !isValid {
		logrus.Warningf("s: %s; invalid unit", s)
		return def
	}
	q, err := resource.ParseQuantity(s)
	if err != nil {
		logrus.Warningf("s: %s; failed to parse quantity: %v", s, err)
		return def
	}
	re, ok := q.AsInt64()
	if !ok {
		logrus.Warningf("failed to int64: %d", re)
		return def
	}
	if re != 0 {
		return int(re) / (1024 * 1024)
	}
	return def
}

//ParseImageName parse image name
func ParseImageName(s string) (i Image) {
	ref, err := reference.ParseAnyReference(s)
	if err != nil {
		logrus.Errorf("image name: %s; parse image failure %s", s, err.Error())
		return i
	}
	name, err := reference.ParseNamed(ref.String())
	if err != nil {
		logrus.Errorf("parse image failure %s", err.Error())
		return i
	}
	i.name = name
	i.Tag = sources.GetTagFromNamedRef(name)
	if strings.Contains(s, ":") {
		i.Name = s[:len(s)-(len(i.Tag)+1)]
	} else {
		i.Name = s
	}
	i.source = s
	return
}
