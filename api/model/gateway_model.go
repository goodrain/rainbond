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

package model

//AddHTTPRuleStruct is used to add http rule, certificate and rule extensions
type AddHTTPRuleStruct struct {
	HTTPRuleID     string                 `json:"http_rule_id" validate:"http_rule_id|required"`
	ServiceID      string                 `json:"service_id" validate:"service_id|required"`
	ContainerPort  int                    `json:"container_port" validate:"container_port|required"`
	Domain         string                 `json:"domain" validate:"domain|required"`
	Path           string                 `json:"path"`
	Header         string                 `json:"header"`
	Cookie         string                 `json:"cookie"`
	Weight         int                    `json:"weight"`
	IP             string                 `json:"ip"`
	CertificateID  string                 `json:"certificate_id"`
	Certificate    string                 `json:"certificate"`
	PrivateKey     string                 `json:"private_key"`
	RuleExtensions []*RuleExtensionStruct `json:"rule_extensions"`
}

//UpdateHTTPRuleStruct is used to update http rule, certificate and rule extensions
type UpdateHTTPRuleStruct struct {
	HTTPRuleID     string                 `json:"http_rule_id" validate:"http_rule_id|required"`
	ServiceID      string                 `json:"service_id"`
	ContainerPort  int                    `json:"container_port"`
	Domain         string                 `json:"domain"`
	Path           string                 `json:"path"`
	Header         string                 `json:"header"`
	Cookie         string                 `json:"cookie"`
	Weight         int                    `json:"weight"`
	IP             string                 `json:"ip"`
	CertificateID  string                 `json:"certificate_id"`
	Certificate    string                 `json:"certificate"`
	PrivateKey     string                 `json:"private_key"`
	RuleExtensions []*RuleExtensionStruct `json:"rule_extensions"`
}

//DeleteHTTPRuleStruct contains the id of http rule that will be deleted
type DeleteHTTPRuleStruct struct {
	HTTPRuleID string `json:"http_rule_id" validate:"http_rule_id|required"`
}

// AddTCPRuleStruct is used to add tcp rule and rule extensions
type AddTCPRuleStruct struct {
	TCPRuleID      string                 `json:"tcp_rule_id" validate:"tcp_rule_id|required"`
	ServiceID      string                 `json:"service_id" validate:"service_id|required"`
	ContainerPort  int                    `json:"container_port"`
	IP             string                 `json:"ip"`
	Port           int                    `json:"port" validate:"service_id|required"`
	RuleExtensions []*RuleExtensionStruct `json:"rule_extensions"`
}

// UpdateTCPRuleStruct is used to update tcp rule and rule extensions
type UpdateTCPRuleStruct struct {
	TCPRuleID      string                 `json:"tcp_rule_id" validate:"tcp_rule_id|required"`
	ServiceID      string                 `json:"service_id"`
	ContainerPort  int                    `json:"container_port"`
	IP             string                 `json:"ip"`
	Port           int                    `json:"port"`
	RuleExtensions []*RuleExtensionStruct `json:"rule_extensions"`
}

// DeleteTCPRuleStruct is used to delete tcp rule and rule extensions
type DeleteTCPRuleStruct struct {
	TCPRuleID string `json:"tcp_rule_id" validate:"tcp_rule_id|required"`
}

// RuleExtensionStruct represents rule extensions for http rule or tcp rule
type RuleExtensionStruct struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// IPPoolStruct contains request data for AddIPPool or UpdateIPPool
type IPPoolStruct struct {
	EID string `json:"eid" validate:"eid|required"`
	CIDR string `json:"cidr" validate:"cidr|required"`
}