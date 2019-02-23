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

// AddEndpiontsReq is one of the Endpoints in the request to add the endpints.
type AddEndpiontsReq struct {
	IP string `json:"ip" validate:"required|ip_v4"`
	IsOnline bool `json:"is_online" validate:"required"`
}

// UpdEndpiontsReq is one of the Endpoints in the request to update the endpints.
type UpdEndpiontsReq struct {
	UUID string `json:"uuid" validate:"required|len:32"`
	IP string `json:"ip" validate:"required|ip_v4"`
	IsOnline bool `json:"is_online" validate:"required"`
}

// DelEndpiontsReq is one of the Endpoints in the request to update the endpints.
type DelEndpiontsReq struct {
	UUID string `json:"uuid" validate:"required|len:32"`
}