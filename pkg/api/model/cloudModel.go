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

//GetUserToken GetUserToken
//swagger:parameters createToken
type GetUserToken struct {
	// in: body
	Body struct {
		// eid
		// in: body
		// required: true
		EID string `json:"eid" validate:"eid|required"`
		// 可控范围:all_power|node_manager|server_source
		// in: body
		// required: false
		Range string `json:"range" validate:"range"`
		// 有效期
		// in: body
		// required: true
		ValidityPeriod int `json:"validity_period" validate:"validity_period|required"` //1549812345
		// 数据中心标识
		// in: body
		// required: false
		RegionTag  string `json:"region_tag" validate:"region_tag"`
		BeforeTime int    `json:"before_time"`
	}
}

//GetTokenInfo GetTokenInfo
//swagger:parameters getTokenInfo
type GetTokenInfo struct {
	// in: path
	// required: true
	EID string `json:"eid" validate:"eid|required"`
}

//UpdateToken UpdateToken
//swagger:parameters updateToken
type UpdateToken struct {
	// in: path
	// required: true
	EID string `json:"eid" validate:"eid|required"`
	//in: body
	Body struct {
		// 有效期
		// in: body
		// required: true
		ValidityPeriod int `json:"validity_period" validate:"validity_period|required"` //1549812345
	}
}

//TokenInfo TokenInfo
type TokenInfo struct {
	EID   string `json:"eid"`
	Token string `json:"token"`
	CA    string `json:"ca"`
	Key   string `json:"key"`
}

//APIManager APIManager
//swagger:parameters addAPIManager deleteAPIManager
type APIManager struct {
	//in: body
	Body struct {
		//api级别
		//in: body
		//required: true
		ClassLevel string `json:"class_level" validate:"class_level|reqired"`
		//uri前部
		//in: body
		//required: true
		Prefix string `json:"prefix" validate:"prefix|required"`
		//完整uri
		//in: body
		//required: false
		URI string `json:"uri" validate:"uri"`
		//别称
		//in: body
		//required: false
		Alias string `json:"alias" validate:"alias"`
		//补充信息
		//in:body
		//required: false
		Remark string `json:"remark" validate:"remark"`
	}
}
