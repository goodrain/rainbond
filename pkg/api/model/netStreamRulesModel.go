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

package model

//SetNetDownStreamRuleStruct SetNetDownStreamRuleStruct
//swagger:parameters setNetDownStreamRuleStruct
type SetNetDownStreamRuleStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		//in: body
		//required: true
		DestService string `json:"dest_service" validate:"dest_service"`
		//下游服务别名
		//in: body
		//required: true
		DestServiceAlias string `json:"dest_service_alias" validate:"dest_service_alias"`
		//端口
		//in: body
		//required: true
		Port int `json:"port" validate:"port"`
		//协议
		//in: body
		//required: true
		Protocol string `json:"protocol" validate:"protocol|between:tcp,http"`
		//规则体
		//in: body
		//required: true
		Rules *NetDownStreamRules `json:"rules" validate:"rules"`
	}
}

//NetRulesDownStreamBody NetRulesDownStreamBody
type NetRulesDownStreamBody struct {
	DestService      string              `json:"dest_service"`
	DestServiceAlias string              `json:"dest_service_alias"`
	Port             int                 `json:"port"`
	Protocol         string              `json:"protocol"`
	Rules            *NetDownStreamRules `json:"rules"`
}

//NetDownStreamRules NetDownStreamRules
type NetDownStreamRules struct {
	//限流值 max_connections
	Limit              int `json:"limit" validate:"limit|numeric_between:0,1024"`
	MaxPendingRequests int `json:"max_pending_requests"`
	MaxRequests        int `json:"max_requests"`
	MaxRetries         int `json:"max_retries"`
	//请求头
	//in: body
	//required: false
	Header []HeaderRules `json:"header" validate:"header"`
	//域名转发
	//in: body
	//required: false
	Domain []string `json:"domain" validate:"domain"`
	//path规则
	//in: body
	//required: false
	Prefix       string `json:"prefix" validate:"prefix"`
	ServiceAlias string `json:"service_alias"`
	ServiceID    string `json:"service_id" validate:"service_id"`
}

//NetUpStreamRules NetUpStreamRules
type NetUpStreamRules struct {
	NetDownStreamRules
	SourcePort int32 `json:"source_port"`
	MapPort    int32 `json:"map_port"`
}

//HeaderRules HeaderRules
type HeaderRules struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

//GetNetDownStreamRuleStruct GetNetDownStreamRuleStruct
//swagger:parameters getNetDownStreamRuleStruct
type GetNetDownStreamRuleStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias" validate:"service_alias"`
	// in: path
	// required: true
	DestServiceAlias string `json:"dest_service_alias" validate:"dest_service_alias"`
	// in: path
	// required: true
	Port int `json:"port" validate:"port|numeric_between:1,65535"`
}

//UpdateNetDownStreamRuleStruct UpdateNetDownStreamRuleStruct
//swagger:parameters updateNetDownStreamRuleStruct
type UpdateNetDownStreamRuleStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias" validate:"service_alias"`
	// in: path
	// required: true
	DestServiceAlias string `json:"dest_service_alias" validate:"dest_service_alias"`
	// in: path
	// required: true
	Port int `json:"port" validate:"port|numeric_between:1,65535"`
	// in: body
	Body struct {
		//in: body
		//required: true
		DestService string `json:"dest_service" validate:"dest_service"`
		//协议
		//in: body
		//required: true
		Protocol string `json:"protocol" validate:"protocol|between:tcp,http"`
		//规则体
		//in: body
		//required: true
		Rules *NetDownStreamRules `json:"rules" validate:"rules"`
	}
}
