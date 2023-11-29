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
	"github.com/goodrain/rainbond/builder/parser/discovery"
	"github.com/goodrain/rainbond/event"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

// VMServiceParse is one of the implematation of parser.Parser
type VMServiceParse struct {
	sourceBody string

	endpoints []*discovery.Endpoint

	errors []ParseError
	logger event.Logger
}

// CreateVMServiceParse creates a new CreateVMServiceParse.
func CreateVMServiceParse(sourceBody string, logger event.Logger) Parser {
	return &VMServiceParse{
		sourceBody: sourceBody,
		logger:     logger,
	}
}

// Parse blablabla
func (t *VMServiceParse) Parse() ParseErrorList {
	if t.sourceBody == "" {
		return []ParseError{}
	}
	var fileExt string
	if strings.HasPrefix(t.sourceBody, "/grdata") {
		fileInfoList, err := ioutil.ReadDir(t.sourceBody)
		if err != nil {
			logrus.Errorf("read package path %v failure: %v", t.sourceBody, err)
			t.errappend(Errorf(FatalError, "http get failure"))
			return t.errors
		}
		if len(fileInfoList) != 1 {
			logrus.Errorf("the current directory contains multiple files: %v", t.sourceBody)
			t.logger.Error("镜像只可以拥有一个，当前上传了多个文件", map[string]string{"step": "parse"})
			t.errappend(Errorf(FatalError, "http get failure"))
			return t.errors
		}
		fileExt = path.Ext(fileInfoList[0].Name())
	} else {
		rsp, err := http.Get(t.sourceBody)
		if err != nil {
			logrus.Errorf("http get %v failure: %v", t.sourceBody, err)
			t.errappend(Errorf(FatalError, "http get failure"))
			return t.errors
		}
		if rsp.StatusCode != http.StatusOK {
			logrus.Errorf("url %v cannot be accessed", t.sourceBody)
			t.logger.Error("镜像下载地址不可用", map[string]string{"step": "parse"})
			t.errappend(Errorf(FatalError, "url address cannot be accessed"))
			return t.errors
		}
		defer func() {
			_ = rsp.Body.Close()
		}()

		baseURL := filepath.Base(t.sourceBody)
		fileName := strings.Split(baseURL, "?")[0]
		fileExt = path.Ext(fileName)
	}
	if fileExt != ".iso" && fileExt != ".qcow2" && fileExt != ".img" && fileExt != ".tar" && fileExt != ".gz" && fileExt != ".xz" {
		t.logger.Error("上传包格式校验失败，不符合包要求", map[string]string{"step": "parse"})
		t.errappend(Errorf(FatalError, "image package format verification failed"))
		return t.errors
	}
	return []ParseError{}
}

// GetServiceInfo returns information of third-party service from
// the receiver *ThirdPartyServiceParse.
func (t *VMServiceParse) GetServiceInfo() []ServiceInfo {
	serviceInfo := ServiceInfo{
		Image: t.GetImage(),
	}
	return []ServiceInfo{serviceInfo}
}

// GetImage is a dummy method. there is no image for Third-party service.
func (t *VMServiceParse) GetImage() Image {
	return Image{}
}

func (t *VMServiceParse) errappend(pe ParseError) {
	t.errors = append(t.errors, pe)
}
