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

package exector

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/pkg/builder/parser"
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/pquerna/ffjson/ffjson"
)

//ServiceCheckInput 任务输入数据
type ServiceCheckInput struct {
	CheckUUID string `json:"uuid"`
	//检测来源类型
	SourceType string `json:"source_type"`

	// 检测来源定义，
	// 代码： https://github.com/shurcooL/githubql.git master
	// docker-run: docker run --name xxx nginx:latest nginx
	// docker-compose: compose全文
	SourceBody string `json:"source_body"`
	TenantID   string
	EventID    string `json:"event_id"`
}

//ServiceCheckResult 应用检测结果
type ServiceCheckResult struct {
	//检测状态 Success Failure
	CheckStatus string `json:"check_status"`
	ErrorInfos  parser.ParseErrorList
	ServiceInfo []parser.ServiceInfo `json:"service_info"`
}

//CreateResult 创建检测结果
func CreateResult(ErrorInfos parser.ParseErrorList, ServiceInfo []parser.ServiceInfo) error {
	var sr ServiceCheckResult
	if ErrorInfos != nil && ErrorInfos.IsFatalError() {
		sr = ServiceCheckResult{
			CheckStatus: "Failure",
			ErrorInfos:  ErrorInfos,
			ServiceInfo: ServiceInfo,
		}
	} else {
		sr = ServiceCheckResult{
			CheckStatus: "Success",
			ErrorInfos:  ErrorInfos,
			ServiceInfo: ServiceInfo,
		}
	}
	//save result
	fmt.Println(sr)
	return nil
}

//serviceCheck 应用创建源检测
func (e *exectorManager) serviceCheck(in []byte) {
	//step1 判断应用源类型
	//step2 获取应用源介质，镜像Or源码
	//step3 解析判断应用源规范
	//完成
	var input ServiceCheckInput
	if err := ffjson.Unmarshal(in, &input); err != nil {
		logrus.Error("Unmarshal service check input data error.", err.Error())
		return
	}
	logger := event.GetManager().GetLogger(input.EventID)
	logger.Info("开始应用构建源检测", map[string]string{"step": "starting"})
	logrus.Infof("start check service by type: %s ", input.SourceType)
	var pr parser.Parser
	switch input.SourceType {
	case "docker-run":
		pr = parser.CreateDockerRunOrImageParse(input.SourceBody, e.DockerClient, logger)
	case "docker-compose":
		pr = parser.CreateDockerComposeParse(input.SourceBody, e.DockerClient, logger)
	case "sourcecode":
		pr = parser.CreateSourceCodeParse(input.SourceBody, logger)
	}
	if pr == nil {
		logger.Error("创建应用来源类型不支持。", map[string]string{"step": "callback", "status": "failure"})
		return
	}
	errList := pr.Parse()
	if errList != nil {
		for i, err := range errList {
			if err.SolveAdvice == "" {
				errList[i].SolveAdvice = fmt.Sprintf("解析器认为镜像名为:%s,请确认是否正确或镜像是否存在", pr.GetImage())
			}
		}
	}
	serviceInfos := pr.GetServiceInfo()
	if err := CreateResult(errList, serviceInfos); err != nil {
		logrus.Errorf("create check result error,%s", err.Error())
		logger.Error("创建检测结果失败。", map[string]string{"step": "callback", "status": "failure"})
	}
	logger.Error("创建检测结果成功。", map[string]string{"step": "latest", "status": "success"})
}
