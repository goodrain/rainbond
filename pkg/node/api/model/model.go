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

import (
	"github.com/pquerna/ffjson/ffjson"
	"k8s.io/client-go/pkg/api/v1"
)

//Resource 资源
type Resource struct {
	CpuR int `json:"cpu"`
	MemR int `json:"mem"`
}

type FirstConfig struct {
	StorageMode     string `json:"storage_mode"`
	StorageHost     string `json:"storage_host,omitempty"`
	StorageEndPoint string `json:"storage_endpoint,omitempty"`

	NetworkMode string `json:"network_mode"`
	ZKHosts     string `json:"zk_host,omitempty"`
	CassandraIP string `json:"cassandra_ip,omitempty"`
	K8SAPIAddr  string `json:"k8s_apiserver,omitempty"`
	MasterIP    string `json:"master_ip,omitempty"`
	DNS         string `json:"dns,omitempty"`
	ZMQSub      string `json:"zmq_sub,omitempty"`
	ZMQTo       string `json:"zmq_to,omitempty"`
	EtcdIP      string `json:"etcd_ip,omitempty"`
}

type Config struct {
	Cn    string `json:"cn_name"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

//ConfigUnit 一个配置单元
type ConfigUnit struct {
	//配置名称 例如:network
	Name   string `json:"name" validate:"name|required"`
	CNName string `json:"cn_name" validate:"cn_name"`
	//类型 例如:midonet
	Type string `json:"type" validate:"name|required"`
	//可选类型 类型名称和需要的配置项
	OptionalType map[string][]map[string]string `json:"optional_type,omitempty"`
	Opts         map[string]interface{}         `json:"opts"`
	//是否用户可配置
	IsConfigurable bool `json:"is_configurable"`
}

//GlobalConfig 全局配置
type GlobalConfig struct {
	NetWork            ConfigUnit `json:"network"`
	Storage            ConfigUnit `json:"storage"`
	Etcd               ConfigUnit `json:"etcd"`
	ManagerDB          ConfigUnit `json:"manager_db"`
	KubeAPI            ConfigUnit `json:"kube_api"`
	DNS                ConfigUnit `json:"dns"`
	ManagerNodeAddress ConfigUnit `json:"mananger_node_addr"`
	ExportLB           ConfigUnit `json:"export_lb"`
	Install            ConfigUnit `json:"install"`
}

//String String
func (g GlobalConfig) String() string {
	res, _ := ffjson.Marshal(&g)
	return string(res)
}

//Bytes Bytes
func (g GlobalConfig) Bytes() []byte {
	res, _ := ffjson.Marshal(&g)
	return res
}

//CreateDefaultGlobalConfig 生成默认配置
func CreateDefaultGlobalConfig() *GlobalConfig {
	var gconfig GlobalConfig
	gconfig.NetWork = ConfigUnit{
		Name:   "network",
		CNName: "集群网络",
		Type:   "calico",
		OptionalType: map[string][]map[string]string{
			"calico": []map[string]string{map[string]string{"name": "ETCD_ADDRS", "name_cn": "ETCD地址", "value_type": "string"}},
			"midonet": []map[string]string{
				map[string]string{"name": "CASSANDRA_ADDRS", "name_cn": "CASSANDRA地址", "value_type": "array"},
				map[string]string{"name": "ZOOKEEPER_ADDRS", "name_cn": "ZOOKEEPER地址", "value_type": "array"},
				map[string]string{"name": "LB_CIDR", "name_cn": "负载均衡所在网段", "value_type": "string"},
			}},
		IsConfigurable: true,
		Opts:           make(map[string]interface{}),
	}
	gconfig.Storage = ConfigUnit{
		Name:   "storage",
		Type:   "nfs",
		CNName: "默认共享存储",
		OptionalType: map[string][]map[string]string{
			"nfs": []map[string]string{
				map[string]string{"name": "NFS_SERVERS", "name_cn": "NFS服务端地址列表", "value_type": "array"},
				map[string]string{"name": "NFS_ENDPOINT", "name_cn": "NFS挂载端点", "value_type": "string"},
			},
			"clusterfs": []map[string]string{},
		},
		IsConfigurable: true,
		Opts:           make(map[string]interface{}),
	}
	gconfig.ManagerDB = ConfigUnit{
		Name:   "manager_db",
		Type:   "mysql",
		CNName: "管理节点数据库类型",
		OptionalType: map[string][]map[string]string{
			"mysql": []map[string]string{
				map[string]string{"name": "MYSQL_HOST", "name_cn": "Mysql数据库地址", "value_type": "string"},
				map[string]string{"name": "MYSQL_PASS", "name_cn": "Mysql数据库密码", "value_type": "string"},
				map[string]string{"name": "MYSQL_USER", "name_cn": "Mysql数据库用户名", "value_type": "string"},
			},
			"cockroachdb": []map[string]string{
				map[string]string{"name": "COCKROACH_HOST", "name_cn": "Mysql数据库地址", "value_type": "array"},
				map[string]string{"name": "COCKROACH_PASS", "name_cn": "Mysql数据库密码", "value_type": "string"},
				map[string]string{"name": "COCKROACH_USER", "name_cn": "Mysql数据库用户名", "value_type": "string"},
			},
		},
		IsConfigurable: true,
		Opts:           make(map[string]interface{}),
	}
	gconfig.ExportLB = ConfigUnit{
		Name:   "export_lb",
		Type:   "nginx",
		CNName: "边缘负载均衡",
		OptionalType: map[string][]map[string]string{
			"nginx": []map[string]string{
				map[string]string{"name": "DOMAIN", "name_cn": "应用域名", "value_type": "string"},
				map[string]string{"name": "INSTALL_NODE", "name_cn": "安装节点", "value_type": "array"},
			},
		},
		IsConfigurable: true,
		Opts:           make(map[string]interface{}),
	}
	gconfig.Install = ConfigUnit{
		Name:   "install",
		Type:   "online",
		CNName: "安装模式",
		OptionalType: map[string][]map[string]string{
			"online":  []map[string]string{},
			"offline": []map[string]string{},
		},
		IsConfigurable: true,
		Opts:           make(map[string]interface{}),
	}
	gconfig.DNS = ConfigUnit{
		Name:   "dns",
		Type:   "gr-dns",
		CNName: "集群DNS服务",
		OptionalType: map[string][]map[string]string{
			"gr-dns": []map[string]string{
				map[string]string{"name": "DNS_HOST", "name_cn": "DNS服务地址", "value_type": "array"},
			},
		},
		Opts: make(map[string]interface{}),
	}
	gconfig.Etcd = ConfigUnit{
		Name:   "etcd",
		Type:   "etcd",
		CNName: "集群ETCD服务",
		OptionalType: map[string][]map[string]string{
			"etcd": []map[string]string{
				map[string]string{"name": "ETCD_ADDR", "name_cn": "ETCD服务地址", "value_type": "array"},
			},
		},
		Opts: make(map[string]interface{}),
	}
	gconfig.KubeAPI = ConfigUnit{
		Name:   "kube-api",
		Type:   "kube-api",
		CNName: "KubernetesAPI服务",
		OptionalType: map[string][]map[string]string{
			"kube-api": []map[string]string{
				map[string]string{"name": "KUBE_ADDR", "name_cn": "KUBE-API服务地址", "value_type": "array"},
			},
		},
		Opts: make(map[string]interface{}),
	}
	gconfig.ManagerNodeAddress = ConfigUnit{
		Name:   "mananger-node-address",
		Type:   "hostIP",
		CNName: "管理节点",
		OptionalType: map[string][]map[string]string{
			"hostIP": []map[string]string{
				map[string]string{"name": "NODE_LIST", "name_cn": "管理节点地址", "value_type": "array"},
				map[string]string{"name": "API_PORT", "name_cn": "API端口", "value_type": "int"},
			},
		},
		Opts: make(map[string]interface{}),
	}
	return &gconfig
}

//CreateGlobalConfig 生成配置
func CreateGlobalConfig(data []byte) (*GlobalConfig, error) {
	var dgc GlobalConfig
	err := ffjson.Unmarshal(data, &dgc)
	if err != nil {
		return nil, err
	}
	return &dgc, nil
}

type LoginResult struct {
	HostPort  string `json:"hostport"`
	LoginType bool   `json:"type"`
	Result    string `json:"result"`
}
type Login struct {
	HostPort  string `json:"hostport"`
	LoginType bool   `json:"type"`
	HostType  string `json:"hosttype"`
	RootPwd   string `json:"pwd,omitempty"`
}
type Body struct {
	List interface{} `json:"list"`
	Bean interface{} `json:"bean,omitempty"`
}
type ResponseBody struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	MsgCN string `json:"msgcn"`
	Body  Body   `json:"body,omitempty"`
}
type Pods struct {
	Namespace       string `json:"namespace"`
	Id              string `json:"id"`
	Name            string `json:"name"`
	CPURequests     string `json:"cpurequest"`
	CPURequestsR    string `json:"cpurequestr"`
	CPULimits       string `json:"cpulimits"`
	CPULimitsR      string `json:"cpulimitsr"`
	MemoryRequests  string `json:"memoryrequests"`
	MemoryRequestsR string `json:"memoryrequestsr"`
	MemoryLimits    string `json:"memorylimits"`
	MemoryLimitsR   string `json:"memorylimitsr"`
}

//NodeDetails NodeDetails
type NodeDetails struct {
	Name               string              `json:"name"`
	Role               []string            `json:"role"`
	Status             string              `json:"status"`
	Labels             map[string]string   `json:"labels"`
	Annotations        map[string]string   `json:"annotations"`
	CreationTimestamp  string              `json:"creationtimestamp"`
	Conditions         []v1.NodeCondition  `json:"conditions"`
	Addresses          map[string]string   `json:"addresses"`
	Capacity           map[string]string   `json:"capacity"`
	Allocatable        map[string]string   `json:"allocatable"`
	SystemInfo         v1.NodeSystemInfo   `json:"systeminfo"`
	ExternalID         string              `json:"externalid"`
	NonterminatedPods  []*Pods             `json:"nonterminatedpods"`
	AllocatedResources map[string]string   `json:"allocatedresources"`
	Events             map[string][]string `json:"events"`
}
