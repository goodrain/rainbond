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

package handler

import (
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/api/util"
	"github.com/goodrain/rainbond/pkg/db"
	node_model "github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/pkg/api/v1"
)

//DiscoverAction DiscoverAction
type DiscoverAction struct{}

//CreateDiscoverActionManager CreateDiscoverActionManager
func CreateDiscoverActionManager(conf *option.Conf) (*DiscoverAction, error) {
	return &DiscoverAction{}, nil
}

//DiscoverService DiscoverService
func (d *DiscoverAction) DiscoverService(serviceInfo string) (*node_model.SDS, *util.APIHandleError) {
	mm := strings.Split(serviceInfo, "_")
	if len(mm) != 3 {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("service_name is not in good format"))
	}
	tenantName := mm[0]
	serviceAlias := mm[1]
	//deployVersion := mm[2]

	namespace, err := d.ToolsGetTenantUUID(tenantName)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get tenant uuid ", err)
	}
	// serviceID, err := d.ToolsGetServiceID(uuid, serviceAlias)
	// if err != nil {
	// return nil, util.CreateAPIHandleErrorFromDBError("get service id ", err)
	// }
	labelname := fmt.Sprintf("name=%sService", serviceAlias)
	endpoint, err := k8s.K8S.Core().Endpoints(namespace).List(metav1.ListOptions{LabelSelector: labelname})
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	services, err := k8s.K8S.Core().Services(namespace).List(metav1.ListOptions{LabelSelector: labelname})
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	if len(endpoint.Items) == 0 {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("have no endpoints"))
	}
	var sdsL []*node_model.PieceSDS
	for key, item := range endpoint.Items {
		addressList := item.Subsets[0].Addresses
		if len(addressList) == 0 {
			addressList = item.Subsets[0].NotReadyAddresses
		}
		//port := item.Subsets[0].Ports[0].Port
		toport := services.Items[key].Spec.Ports[0].Port
		for _, ip := range addressList {
			sdsP := &node_model.PieceSDS{
				IPAddress: ip.IP,
				Port:      toport,
			}
			sdsL = append(sdsL, sdsP)
		}
	}
	sds := &node_model.SDS{
		Hosts: sdsL,
	}
	return sds, nil
}

//ToolsGetTenantUUID GetTenantUUID
func (d *DiscoverAction) ToolsGetTenantUUID(namespace string) (string, error) {
	tenants, err := db.GetManager().TenantDao().GetTenantIDByName(namespace)
	if err != nil {
		return "", err
	}
	return tenants.UUID, nil
}

//ToolsGetServiceID GetServiceID
func (d *DiscoverAction) ToolsGetServiceID(uuid, serviceAlias string) (string, error) {
	services, err := db.GetManager().TenantServiceDao().GetServiceByTenantIDAndServiceAlias(uuid, serviceAlias)
	if err != nil {
		return "", err
	}
	return services.ServiceID, nil
}

//ToolsGetK8SServiceList GetK8SServiceList
func (d *DiscoverAction) ToolsGetK8SServiceList(uuid string) (*v1.ServiceList, error) {
	serviceList, err := k8s.K8S.Core().Services(uuid).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return serviceList, nil
}
