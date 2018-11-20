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

func (Certificate) TableName() string {
	return "gateway_certificate"
}

// Certificate contains TLS information
type Certificate struct {
	Model
	UUID            string `gorm:"column:uuid"`
	CertificateName string `gorm:"column:certificate_name;size:128"`
	Certificate     string `gorm:"column:certificate;size:128"`
	PrivateKey      string `gorm:"column:private_key;size:128"`
}

func (RuleExtension) TableName() string {
	return "gateway_rule_extension"
}

type ExtensionValue string

var HttpToHttpsEV ExtensionValue = "HttpToHttps"

type RuleExtension struct {
	Model
	ServiceID string         `gorm:"column:service_id"`
	Value     ExtensionValue `gorm:"column:rule_value_type"`
}

type LoadBalancerType string

var RoundRobinLBType LoadBalancerType = "RoundRobin"

var ConsistenceHashLBType LoadBalancerType = "ConsistentHash"

func (HttpRule) TableName() string {
	return "gateway_http_rule"
}

// HttpRule contains http rule
type HttpRule struct {
	Model
	ServiceID        string           `gorm:"column:service_id"`
	ContainerPort    int              `gorm:"column:container_port"`
	Domain           string           `gorm:"column:domain"`
	Path             string           `gorm:"column:path"`
	Header           string           `gorm:"column:header"`
	Cookie           string           `gorm:"column:cookie"`
	IP               string           `gorm:"column:ip"`
	LoadBalancerType string `gorm:"column:load_balancer_type"`
	CertificateID    string           `gorm:"column:certificate_id"`
}

func (TcpRule) TableName() string {
	return "gateway_tcp_rule"
}

// TcpRule contain stream rule
type TcpRule struct {
	Model
	ServiceID        string           `gorm:"column:service_id"`
	ContainerPort    int              `gorm:"column:container_port"`
	IP               string           `gorm:"column:ip"`
	Port             int              `gorm:"column:port"` // TODO: 这个就是mappingPort吗???
	LoadBalancerType string `gorm:"column:load_balancer_type"`
}
