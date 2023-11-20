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
