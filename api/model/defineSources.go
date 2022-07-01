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

//SetDefineSourcesStruct SetDefineSourcesStruct
//swagger:parameters setDefineSource
type SetDefineSourcesStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// in: path
	// required: true
	SourceAlias string `json:"source_alias" validate:"source_alias"`
	// in: body
	Body struct {
		//in: body
		//required: true
		SourceSpec *SourceSpec `json:"source_spec" validate:"source_spec"`
	}
}

//DeleteDefineSourcesStruct DeleteDefineSourcesStruct
//swagger:parameters deleteDefineSource getDefineSource
type DeleteDefineSourcesStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// in: path
	// required: true
	SourceAlias string `json:"source_alias" validate:"source_alias"`
	// in: path
	// required: true
	EnvName string `json:"env_name" validate:"env_name"`
}

//UpdateDefineSourcesStruct UpdateDefineSourcesStruct
//swagger:parameters deleteDefineSource updateDefineSource
type UpdateDefineSourcesStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name" validate:"tenant_name"`
	// in: path
	// required: true
	SourceAlias string `json:"source_alias" validate:"source_alias"`
	// in: path
	// required: true
	EnvName string `json:"env_name" validate:"env_name"`
	// in: body
	Body struct {
		//in: body
		//required: true
		SourceSpec *SourceSpec `json:"source_spec" validate:"source_spec"`
	}
}

//SourceSpec SourceSpec
type SourceSpec struct {
	Alias      string               `json:"source_alias" validate:"source_alias"`
	Info       string               `json:"source_info" validate:"source_info"`
	CreateTime string               `json:"create_time" validate:"create_time"`
	Operator   string               `json:"operator" validate:"operator"`
	SourceBody *SoureBody           `json:"source_body" validate:"source_body"`
	Additions  map[string]*Addition `json:"additons" validate:"additions"`
}

//SoureBody SoureBody
type SoureBody struct {
	EnvName string      `json:"env_name" validate:"env_name"`
	EnvVal  interface{} `json:"env_value" validate:"env_value"`
	//json format
}

//ResourceSpec 资源结构体
type ResourceSpec struct {
	BasePorts    []*BasePort    `json:"base_ports"`
	BaseServices []*BaseService `json:"base_services"`
	BaseNormal   BaseEnv        `json:"base_normal"`
}

//PluginStorage 插件存储结构体
type PluginStorage struct {
	VolumeName  string `json:"volume_name"`
	VolumePath  string `json:"volume_path"`
	FileContent string `json:"file_content"`
	AttrType    string `json:"attr_type"`
}

//BasePort base of current app ports
type BasePort struct {
	ServiceAlias string `json:"service_alias"`
	ServiceID    string `json:"service_id"`
	//Port is the real app port
	Port int `json:"port"`
	//ListenPort is mesh listen port, proxy connetion to real app port
	ListenPort int                    `json:"listen_port"`
	Protocol   string                 `json:"protocol"`
	Options    map[string]interface{} `json:"options"`
}

//BaseService 基于依赖应用及端口结构
type BaseService struct {
	ServiceAlias       string                 `json:"service_alias"`
	ServiceID          string                 `json:"service_id"`
	DependServiceAlias string                 `json:"depend_service_alias"`
	DependServiceID    string                 `json:"depend_service_id"`
	Port               int                    `json:"port"`
	Protocol           string                 `json:"protocol"`
	Options            map[string]interface{} `json:"options"`
}

//BaseEnv 无平台定义类型，普通kv
type BaseEnv struct {
	Options map[string]interface{} `json:"options"`
}

//Item source值,键值对形式
type Item struct {
	Key   string      `json:"key" validate:"key"`
	Value interface{} `json:"value" validate:"value"`
}

//Addition 存储附加信息
type Addition struct {
	Desc  string  `json:"desc" validate:"desc"`
	Items []*Item `json:"items" validate:"items"`
}
