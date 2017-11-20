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

//SetDefineSourcesStruct SetDefineSourcesStruct
//swagger:parameters setDefineSource updateDefineSource
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
//swagger:parameters deleteDefineSource updateDefineSourcesStruct
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
	EnvName string `json:"env_name" validate:"env_name"`
	EnvVal  string `json:"env_value" validate:"env_value"`
	//json format
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
