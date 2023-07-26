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

package v1

//GetCommonLabels get common labels
func (a *AppService) GetCommonLabels(labels ...map[string]string) map[string]string {
	var resultLabel = make(map[string]string)
	for _, l := range labels {
		for k, v := range l {
			resultLabel[k] = v
		}
	}
	if !a.DryRun {
		resultLabel["creater_id"] = a.CreaterID
	}
	resultLabel["creator"] = "Rainbond"
	resultLabel["service_id"] = a.ServiceID
	resultLabel["service_alias"] = a.ServiceAlias
	resultLabel["tenant_name"] = a.TenantName
	resultLabel["tenant_id"] = a.TenantID
	resultLabel["app_id"] = a.AppID
	resultLabel["rainbond_app"] = a.K8sApp
	return resultLabel
}
