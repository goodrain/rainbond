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
	"strings"

	"github.com/goodrain/rainbond/event"
)

// ThirdPartyServiceParse is one of the implematation of parser.Parser
type ThirdPartyServiceParse struct {
	sourceBody string
	errors     []ParseError
	logger     event.Logger
}

// CreateThirdPartyServiceParse creates a new ThirdPartyServiceParse.
func CreateThirdPartyServiceParse(sourceBody string, logger event.Logger) Parser {
	return &ThirdPartyServiceParse{
		sourceBody: sourceBody,
		logger:     logger,
	}
}

// Parse blablabla
func (t *ThirdPartyServiceParse) Parse() ParseErrorList {
	// empty t.sourceBody means the service has static endpoints
	// static endpoints is no need to do service check.
	if strings.Replace(t.sourceBody, " ", "", -1) == "" {
		return nil
	}

	return nil
}

// GetServiceInfo returns information of third-party service from
// the receiver *ThirdPartyServiceParse.
func (t *ThirdPartyServiceParse) GetServiceInfo() []ServiceInfo {
	serviceInfo := ServiceInfo{}
	return []ServiceInfo{serviceInfo}
}

// GetImage is a dummy method. there is no image for Third-party service.
func (t *ThirdPartyServiceParse) GetImage() Image {
	return Image{}
}
