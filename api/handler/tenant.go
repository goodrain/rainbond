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

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/appruntimesync/client"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"

	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"

	"strings"

	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

//TenantAction tenant act
type TenantAction struct {
	MQClient   pb.TaskQueueClient
	KubeClient *kubernetes.Clientset
	statusCli  *client.AppRuntimeSyncClient
}

//CreateTenManager create Manger
func CreateTenManager(MQClient pb.TaskQueueClient, KubeClient *kubernetes.Clientset, statusCli *client.AppRuntimeSyncClient) *TenantAction {
	return &TenantAction{
		MQClient:   MQClient,
		KubeClient: KubeClient,
		statusCli:  statusCli,
	}
}

//GetTenants get tenants
func (t *TenantAction) GetTenants() ([]*dbmodel.Tenants, error) {
	tenants, err := db.GetManager().TenantDao().GetALLTenants()
	if err != nil {
		return nil, err
	}
	return tenants, err
}

//GetTenantsByEid GetTenantsByEid
func (t *TenantAction) GetTenantsByEid(eid string) ([]*dbmodel.Tenants, error) {
	tenants, err := db.GetManager().TenantDao().GetTenantByEid(eid)
	if err != nil {
		return nil, err
	}
	return tenants, err
}

//GetTenantsPaged GetTenantsPaged
func (t *TenantAction) GetTenantsPaged(offset, len int) ([]*dbmodel.Tenants, error) {
	tenants, err := db.GetManager().TenantDao().GetALLTenants()
	if err != nil {
		return nil, err
	}
	return tenants, err
}

//TotalMemCPU StatsMemCPU
func (t *TenantAction) TotalMemCPU(services []*dbmodel.TenantServices) (*api_model.StatsInfo, error) {
	cpus := 0
	mem := 0
	for _, service := range services {
		logrus.Debugf("service is %s, cpus is %v, mem is %v", service.ID, service.ContainerCPU, service.ContainerMemory)
		cpus += service.ContainerCPU
		mem += service.ContainerMemory
	}
	si := &api_model.StatsInfo{
		CPU: cpus,
		MEM: mem,
	}
	return si, nil
}

//GetTenantsName get tenants name
func (t *TenantAction) GetTenantsName() ([]string, error) {
	tenants, err := db.GetManager().TenantDao().GetALLTenants()
	if err != nil {
		return nil, err
	}
	var result []string
	for _, v := range tenants {
		result = append(result, strings.ToLower(v.Name))
	}
	return result, err
}

//GetTenantsByName get tenants
func (t *TenantAction) GetTenantsByName(name string) (*dbmodel.Tenants, error) {
	tenant, err := db.GetManager().TenantDao().GetTenantIDByName(name)
	if err != nil {
		return nil, err
	}

	return tenant, err
}

//GetTenantsByUUID get tenants
func (t *TenantAction) GetTenantsByUUID(uuid string) (*dbmodel.Tenants, error) {
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(uuid)
	if err != nil {
		return nil, err
	}

	return tenant, err
}

//StatsMemCPU StatsMemCPU
func (t *TenantAction) StatsMemCPU(services []*dbmodel.TenantServices) (*api_model.StatsInfo, error) {
	cpus := 0
	mem := 0
	for _, service := range services {
		status := t.statusCli.GetStatus(service.ServiceID)
		if t.statusCli.IsClosedStatus(status) {
			continue
		}
		cpus += service.ContainerCPU
		mem += service.ContainerMemory
	}
	si := &api_model.StatsInfo{
		CPU: cpus,
		MEM: mem,
	}
	return si, nil
}

// QueryResult contains result data for a query.
type QueryResult struct {
	Data struct {
		Type   string                   `json:"resultType"`
		Result []map[string]interface{} `json:"result"`
	} `json:"data"`
	Status string `json:"status"`
}

//GetTenantsResources Gets the resource usage of the specified tenant.
func (t *TenantAction) GetTenantsResources(tr *api_model.TenantResources) (map[string]map[string]interface{}, error) {
	ids, err := db.GetManager().TenantDao().GetTenantIDsByNames(tr.Body.TenantNames)
	if err != nil {
		return nil, err
	}
	services, err := db.GetManager().TenantServiceDao().GetServicesByTenantIDs(ids)
	if err != nil {
		return nil, err
	}
	var serviceIDs []string
	var serviceMap = make(map[string]dbmodel.TenantServices, len(services))
	for _, s := range services {
		serviceIDs = append(serviceIDs, s.ServiceID)
		serviceMap[s.ServiceID] = *s
	}
	var result = make(map[string]map[string]interface{}, len(ids))
	status := t.statusCli.GetStatuss(strings.Join(serviceIDs, ","))
	for k, v := range status {
		if _, ok := serviceMap[k]; !ok {
			continue
		}
		if _, ok := result[serviceMap[k].TenantID]; !ok {
			result[serviceMap[k].TenantID] = map[string]interface{}{"tenant_id": k, "cpu": 0, "memory": 0, "disk": 0}
		}
		if !t.statusCli.IsClosedStatus(v) {
			result[serviceMap[k].TenantID]["cpu"] = result[serviceMap[k].TenantID]["cpu"].(int) + (serviceMap[k].ContainerCPU * serviceMap[k].Replicas)
			result[serviceMap[k].TenantID]["memory"] = result[serviceMap[k].TenantID]["memory"].(int) + (serviceMap[k].ContainerMemory * serviceMap[k].Replicas)
		}
	}
	//query disk used in prometheus
	proxy := GetPrometheusProxy()
	query := fmt.Sprintf(`sum(app_resource_appfs{tenant_id=~"%s"}) by(tenant_id)`, strings.Join(ids, "|"))
	query = strings.Replace(query, " ", "%20", -1)
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:9999/api/v1/query?query=%s", query), nil)
	if err != nil {
		logrus.Error("create request prometheus api error ", err.Error())
		return result, nil
	}
	presult, err := proxy.Do(req)
	if err != nil {
		logrus.Error("do proxy request prometheus api error ", err.Error())
		return result, nil
	}
	if presult.Body != nil {
		defer presult.Body.Close()
		if presult.StatusCode != 200 {
			return result, nil
		}
		var qres QueryResult
		err = json.NewDecoder(presult.Body).Decode(&qres)
		if err == nil {
			for _, re := range qres.Data.Result {
				var tenantID string
				var disk int
				if tid, ok := re["metric"].(map[string]interface{}); ok {
					tenantID = tid["tenant_id"].(string)
				}
				if re, ok := (re["value"]).([]interface{}); ok && len(re) == 2 {
					disk, _ = strconv.Atoi(re[1].(string))
				}
				if _, ok := result[tenantID]; ok {
					result[tenantID]["disk"] = disk / 1024
				}
			}
		}
	}
	return result, nil
}

//GetServicesResources Gets the resource usage of the specified service.
func (t *TenantAction) GetServicesResources(tr *api_model.ServicesResources) (re map[string]map[string]interface{}, err error) {
	status := t.statusCli.GetStatuss(strings.Join(tr.Body.ServiceIDs, ","))
	var running, closed []string
	for k, v := range status {
		if !t.statusCli.IsClosedStatus(v) {
			running = append(running, k)
		} else {
			closed = append(closed, k)
		}
	}
	resmp, err := db.GetManager().TenantServiceDao().GetServiceMemoryByServiceIDs(running)
	if err != nil {
		return nil, err
	}
	for _, c := range closed {
		resmp[c] = map[string]interface{}{"memory": 0, "cpu": 0}
	}
	re = resmp
	appdisks := t.statusCli.GetAppsDisk(strings.Join(tr.Body.ServiceIDs, ","))
	for serviceID, disk := range appdisks {
		if _, ok := resmp[serviceID]; ok {
			resmp[serviceID]["disk"] = disk / 1024
		} else {
			resmp[serviceID] = make(map[string]interface{})
			resmp[serviceID]["disk"] = disk / 1024
		}
	}
	return resmp, nil
}

//TenantsSum TenantsSum
func (t *TenantAction) TenantsSum() (int, error) {
	s, err := db.GetManager().TenantDao().GetALLTenants()
	if err != nil {
		return 0, err
	}
	return len(s), nil
}

//GetProtocols GetProtocols
func (t *TenantAction) GetProtocols() ([]*dbmodel.RegionProcotols, *util.APIHandleError) {
	rps, err := db.GetManager().RegionProcotolsDao().GetAllSupportProtocol("v2")
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get all support protocols", err)
	}
	return rps, nil
}

//TransPlugins TransPlugins
func (t *TenantAction) TransPlugins(tenantID, tenantName, fromTenant string, pluginList []string) *util.APIHandleError {
	tenantInfo, err := db.GetManager().TenantDao().GetTenantIDByName(fromTenant)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("get tenant infos", err)
	}
	goodrainID := tenantInfo.UUID
	tx := db.GetManager().Begin()
	for _, p := range pluginList {
		pluginInfo, err := db.GetManager().TenantPluginDao().GetPluginByID(p, goodrainID)
		if err != nil {
			tx.Rollback()
			return util.CreateAPIHandleErrorFromDBError("get plugin infos", err)
		}
		pluginInfo.TenantID = tenantID
		pluginInfo.Domain = tenantName
		pluginInfo.ID = 0
		err = db.GetManager().TenantPluginDaoTransactions(tx).AddModel(pluginInfo)
		if err != nil {
			if !strings.Contains(err.Error(), "is exist") {
				tx.Rollback()
				return util.CreateAPIHandleErrorFromDBError("add plugin Info", err)
			}
		}
	}
	if err := tx.Commit().Error; err != nil {
		return util.CreateAPIHandleErrorFromDBError("trans plugins infos", err)
	}
	return nil
}
