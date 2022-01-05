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

// TableName returns table name of Certificate
func (Certificate) TableName() string {
	return "gateway_certificate"
}

// Certificate contains TLS information
type Certificate struct {
	Model
	UUID            string `gorm:"column:uuid"`
	CertificateName string `gorm:"column:certificate_name;size:128"`
	Certificate     string `gorm:"column:certificate;size:65535"`
	PrivateKey      string `gorm:"column:private_key;size:65535"`
}

// TableName returns table name of RuleExtension
func (RuleExtension) TableName() string {
	return "gateway_rule_extension"
}

// RuleExtensionKey rule extension key
type RuleExtensionKey string

// HTTPToHTTPS forces http rewrite to https
var HTTPToHTTPS RuleExtensionKey = "httptohttps"

// LBType load balancer type
var LBType RuleExtensionKey = "lb-type"

// RuleExtension contains rule extensions for http rule or tcp rule
type RuleExtension struct {
	Model
	UUID   string `gorm:"column:uuid"`
	RuleID string `gorm:"column:rule_id"`
	Key    string `gorm:"column:key"`
	Value  string `gorm:"column:value"`
}

// LoadBalancerType load balancer type
type LoadBalancerType string

// RoundRobin round-robin load balancer type
var RoundRobin LoadBalancerType = "RoundRobin"

// ConsistenceHash consistence hash load balancer type
var ConsistenceHash LoadBalancerType = "ConsistentHash"

// TableName returns table name of HTTPRule
func (HTTPRule) TableName() string {
	return "gateway_http_rule"
}

// HTTPRule contains http rule
type HTTPRule struct {
	Model
	UUID          string `gorm:"column:uuid"`
	ServiceID     string `gorm:"column:service_id"`
	ContainerPort int    `gorm:"column:container_port"`
	Domain        string `gorm:"column:domain"`
	Path          string `gorm:"column:path;type:longtext"`
	Header        string `gorm:"column:header;type:longtext"`
	Cookie        string `gorm:"column:cookie;type:longtext"`
	Weight        int    `gorm:"column:weight"`
	IP            string `gorm:"column:ip"`
	CertificateID string `gorm:"column:certificate_id"`
	PathRewrite   bool   `gorm:"column:path_rewrite"`
}

// TableName returns table name of TCPRule
func (TCPRule) TableName() string {
	return "gateway_tcp_rule"
}

// HTTPRuleRewrite containe http rule rewrites
type HTTPRuleRewrite struct {
	Model
	UUID        string `gorm:"column:uuid"`
	HTTPRuleID  string `gorm:"column:http_rule_id"`
	Regex       string `gorm:"column:regex"`
	Replacement string `gorm:"column:replacement"`
	Flag        string `gorm:"column:flag"`
}

// TableName retuens table name of HTTPRuleRewrite
func (HTTPRuleRewrite) TableName() string {
	return "gateway_http_rule_rewrite"
}

// TCPRule contain stream rule
type TCPRule struct {
	Model
	UUID          string `gorm:"column:uuid"`
	ServiceID     string `gorm:"column:service_id"`
	ContainerPort int    `gorm:"column:container_port"`
	// external access ip
	IP string `gorm:"column:ip"`
	// external access port
	Port int `gorm:"column:port"`
}

// GwRuleConfig describes a configuration of gateway rule.
type GwRuleConfig struct {
	Model
	RuleID string `gorm:"column:rule_id;size:32"`
	Key    string `gorm:"column:key"`
	Value  string `gorm:"column:value"`
}

// TableName -
func (GwRuleConfig) TableName() string {
	return "gateway_rule_config"
}
