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

//HttpRuleStruct -
type HttpRuleStruct struct {
	ContainerPort    int                    `json:"container_port" validate:"container_port|required"`
	Domain           string                 `json:"domain"`
	Path             string                 `json:"path"`
	Header           string                 `json:"header"`
	Cookie           string                 `json:"cookie"`
	IP               string                 `json:"ip"`
	LoadBalancerType string                 `json:"load_balancer_type"`
	CertificateID    string                 `json:"certificate_id"`
	CertificateName  string                 `json:"certificate_name"`
	Certificate      string                 `json:"certificate"`
	PrivateKey       string                 `json:"private_key"`
	RuleExtensions   []*RuleExtensionStruct `json:"rule_extensions"`
}

type RuleExtensionStruct struct {
	Value string `json:"value"`
}
