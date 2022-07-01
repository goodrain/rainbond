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

import (
	dbmodel "github.com/goodrain/rainbond/db/model"
)

//TenantResList TenantResList
type TenantResList []*TenantResource

//PagedTenantResList PagedTenantResList
type PagedTenantResList struct {
	List   []*TenantResource `json:"list"`
	Length int               `json:"length"`
}

//TenantResource abandoned
type TenantResource struct {
	//without plugin
	AllocatedCPU int `json:"alloc_cpu"`
	//without plugin
	AllocatedMEM int `json:"alloc_memory"`
	//with plugin
	UsedCPU int `json:"used_cpu"`
	//with plugin
	UsedMEM  int     `json:"used_memory"`
	UsedDisk float64 `json:"used_disk"`
	Name     string  `json:"name"`
	UUID     string  `json:"uuid"`
	EID      string  `json:"eid"`
}

func (list TenantResList) Len() int {
	return len(list)
}

func (list TenantResList) Less(i, j int) bool {
	if list[i].UsedMEM > list[j].UsedMEM {
		return true
	} else if list[i].UsedMEM < list[j].UsedMEM {
		return false
	} else {
		return list[i].UsedCPU > list[j].UsedCPU
	}
}

func (list TenantResList) Swap(i, j int) {
	temp := list[i]
	list[i] = list[j]
	list[j] = temp
}

//TenantAndResource tenant and resource strcut
type TenantAndResource struct {
	dbmodel.Tenants
	CPURequest            int64 `json:"cpu_request"`
	CPULimit              int64 `json:"cpu_limit"`
	MemoryRequest         int64 `json:"memory_request"`
	MemoryLimit           int64 `json:"memory_limit"`
	RunningAppNum         int64 `json:"running_app_num"`
	RunningAppInternalNum int64 `json:"running_app_internal_num"`
	RunningAppThirdNum    int64 `json:"running_app_third_num"`
	RunningApplications   int64 `json:"running_applications"`
}

//TenantList Tenant list struct
type TenantList []*TenantAndResource

//Add add
func (list *TenantList) Add(tr *TenantAndResource) {
	*list = append(*list, tr)
}
func (list TenantList) Len() int {
	return len(list)
}

func (list TenantList) Less(i, j int) bool {
	// Highest priority
	if list[i].MemoryRequest > list[j].MemoryRequest {
		return true
	}
	if list[i].MemoryRequest == list[j].MemoryRequest {
		if list[i].CPURequest > list[j].CPURequest {
			return true
		}
		if list[i].CPURequest == list[j].CPURequest {
			if list[i].RunningAppNum > list[j].RunningAppNum {
				return true
			}
			if list[i].RunningAppNum == list[j].RunningAppNum {
				// Minimum priority
				if list[i].Tenants.LimitMemory > list[j].Tenants.LimitMemory {
					return true
				}
			}
		}
	}
	return false
}

func (list TenantList) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

//Paging paging
func (list TenantList) Paging(page, pageSize int) map[string]interface{} {
	startIndex := (page - 1) * pageSize
	endIndex := page * pageSize
	var relist TenantList
	if startIndex < list.Len() && endIndex < list.Len() {
		relist = list[startIndex:endIndex]
	}
	if startIndex < list.Len() && endIndex >= list.Len() {
		relist = list[startIndex:]
	}
	return map[string]interface{}{
		"list":     relist,
		"page":     page,
		"pageSize": pageSize,
		"total":    list.Len(),
	}
}
