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

import "k8s.io/api/core/v1"

//Domain 域名实体
//TenantName+ServiceAlias+ServicePort ->PoolName
type Domain struct {
	UUID         string `json:"uuid"`
	DomainName   string `json:"domain_name"`
	ServiceAlias string `json:"service_alias"`
	TenantID     string `json:"tenant_id"`
	TenantName   string `json:"tenant_name"`
	ServicePort  int32  `json:"service_port"`
	//域名协议处理方式，包括:http https (httptohttps)http转https (httpandhttps)http与https共存
	Protocol        string `json:"protocol"`
	AddTime         string `json:"add_time"`
	AddUser         string `json:"add_user"`
	CertificateName string `json:"certificate_name,omitempty"`
	Certificate     string `json:"certificate,omitempty"`
	PrivateKey      string `json:"private_key,omitempty"`
}

//HostNode 集群节点实体
type HostNode struct {
	UUID            string            `json:"uuid"`
	HostName        string            `json:"host_name"`
	InternalIP      string            `json:"internal_ip"`
	ExternalIP      string            `json:"external_ip"`
	AvailableMemory int64             `json:"available_memory"`
	AvailableCPU    int64             `json:"available_cpu"`
	Role            string            `json:"role"`   //计算节点 or 管理节点
	Status          string            `json:"status"` //节点状态 create,init,running,stop,delete
	Labels          map[string]string `json:"labels"`
	Unschedulable   bool              `json:"unschedulable"` //不可调度
	NodeStatus      *v1.NodeStatus    `json:"node_status,omitempty"`
}
