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
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/goodrain/rainbond/api/client/prometheus"
	"github.com/goodrain/rainbond/api/model"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mqclient "github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	rutil "github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

//TenantAction tenant act
type TenantAction struct {
	MQClient                  mqclient.MQClient
	statusCli                 *client.AppRuntimeSyncClient
	OptCfg                    *option.Config
	kubeClient                *kubernetes.Clientset
	cacheClusterResourceStats *ClusterResourceStats
	cacheTime                 time.Time
	prometheusCli             prometheus.Interface
	k8sClient                 k8sclient.Client
	resources                 map[string]runtime.Object
}

//CreateTenManager create Manger
func CreateTenManager(mqc mqclient.MQClient, statusCli *client.AppRuntimeSyncClient,
	optCfg *option.Config,
	kubeClient *kubernetes.Clientset,
	prometheusCli prometheus.Interface,
	k8sClient k8sclient.Client) *TenantAction {

	resources := map[string]runtime.Object{
		"helmApp": &v1alpha1.HelmApp{},
		"service": &corev1.Service{},
	}

	return &TenantAction{
		MQClient:      mqc,
		statusCli:     statusCli,
		OptCfg:        optCfg,
		kubeClient:    kubeClient,
		prometheusCli: prometheusCli,
		k8sClient:     k8sClient,
		resources:     resources,
	}
}

//BindTenantsResource query tenant resource used and sort
func (t *TenantAction) BindTenantsResource(source []*dbmodel.Tenants) api_model.TenantList {
	var list api_model.TenantList
	var resources = make(map[string]*pb.TenantResource, len(source))
	if len(source) == 1 {
		re, err := t.statusCli.GetTenantResource(source[0].UUID)
		if err != nil {
			logrus.Errorf("get tenant %s resource failure %s", source[0].UUID, err.Error())
		}
		if re != nil {
			resources[source[0].UUID] = re
		}
	} else {
		res, err := t.statusCli.GetAllTenantResource()
		if err != nil {
			logrus.Errorf("get all tenant resource failure %s", err.Error())
		}
		if res != nil {
			resources = res.Resources
		}
	}
	for i, ten := range source {
		var item = &api_model.TenantAndResource{
			Tenants: *source[i],
		}
		re := resources[ten.UUID]
		if re != nil {
			item.CPULimit = re.CpuLimit
			item.CPURequest = re.CpuRequest
			item.MemoryLimit = re.MemoryLimit
			item.MemoryRequest = re.MemoryRequest
			item.RunningAppNum = re.RunningAppNum
			item.RunningAppInternalNum = re.RunningAppInternalNum
			item.RunningAppThirdNum = re.RunningAppThirdNum
		}
		list.Add(item)
	}
	sort.Sort(list)
	return list
}

//GetTenants get tenants
func (t *TenantAction) GetTenants(query string) ([]*dbmodel.Tenants, error) {
	tenants, err := db.GetManager().TenantDao().GetALLTenants(query)
	if err != nil {
		return nil, err
	}
	return tenants, err
}

//GetTenantsByEid GetTenantsByEid
func (t *TenantAction) GetTenantsByEid(eid, query string) ([]*dbmodel.Tenants, error) {
	tenants, err := db.GetManager().TenantDao().GetTenantByEid(eid, query)
	if err != nil {
		return nil, err
	}
	return tenants, err
}

//UpdateTenant update tenant info
func (t *TenantAction) UpdateTenant(tenant *dbmodel.Tenants) error {
	return db.GetManager().TenantDao().UpdateModel(tenant)
}

// DeleteTenant deletes tenant based on the given tenantID.
//
// tenant can only be deleted without service or plugin
func (t *TenantAction) DeleteTenant(ctx context.Context, tenantID string) error {
	// check if there are still services
	services, err := db.GetManager().TenantServiceDao().ListServicesByTenantID(tenantID)
	if err != nil {
		return err
	}
	if len(services) > 0 {
		for _, service := range services {
			GetServiceManager().TransServieToDelete(ctx, tenantID, service.ServiceID)
		}
	}

	// check if there are still plugins
	plugins, err := db.GetManager().TenantPluginDao().ListByTenantID(tenantID)
	if err != nil {
		return err
	}
	if len(plugins) > 0 {
		for _, plugin := range plugins {
			GetPluginManager().DeletePluginAct(plugin.PluginID, tenantID)
		}
	}

	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(tenantID)
	if err != nil {
		return err
	}
	oldStatus := tenant.Status
	var rollback = func() {
		tenant.Status = oldStatus
		_ = db.GetManager().TenantDao().UpdateModel(tenant)
	}
	tenant.Status = dbmodel.TenantStatusDeleting.String()
	if err := db.GetManager().TenantDao().UpdateModel(tenant); err != nil {
		return err
	}

	// delete namespace in k8s
	err = t.MQClient.SendBuilderTopic(mqclient.TaskStruct{
		TaskType: "delete_tenant",
		Topic:    mqclient.WorkerTopic,
		TaskBody: map[string]string{
			"tenant_id": tenantID,
		},
	})
	if err != nil {
		rollback()
		logrus.Error("send task 'delete tenant'", err)
		return err
	}

	return nil
}

//TotalMemCPU StatsMemCPU
func (t *TenantAction) TotalMemCPU(services []*dbmodel.TenantServices) (*api_model.StatsInfo, error) {
	cpus := 0
	mem := 0
	for _, service := range services {
		logrus.Debugf("service is %d, cpus is %d, mem is %v", service.ID, service.ContainerCPU, service.ContainerMemory)
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
	tenants, err := db.GetManager().TenantDao().GetALLTenants("")
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
func (t *TenantAction) GetTenantsResources(ctx context.Context, tr *api_model.TenantResources) (map[string]map[string]interface{}, error) {
	ids, err := db.GetManager().TenantDao().GetTenantIDsByNames(tr.Body.TenantNames)
	if err != nil {
		return nil, err
	}
	limits, err := db.GetManager().TenantDao().GetTenantLimitsByNames(tr.Body.TenantNames)
	if err != nil {
		return nil, err
	}
	services, err := db.GetManager().TenantServiceDao().GetServicesByTenantIDs(ids)
	if err != nil {
		return nil, err
	}
	var serviceTenantCount = make(map[string]int, len(ids))
	for _, s := range services {
		serviceTenantCount[s.TenantID]++
	}
	// get cluster resources
	clusterStats, err := t.GetAllocatableResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting allocatalbe cpu and memory: %v", err)
	}
	var result = make(map[string]map[string]interface{}, len(ids))
	var resources = make(map[string]*pb.TenantResource, len(ids))
	if len(ids) == 1 {
		re, err := t.statusCli.GetTenantResource(ids[0])
		if err != nil {
			logrus.Errorf("get tenant %s resource failure %s", ids[0], err.Error())
		}
		if re != nil {
			resources[ids[0]] = re
		}
	} else {
		res, err := t.statusCli.GetAllTenantResource()
		if err != nil {
			logrus.Errorf("get all tenant resource failure %s", err.Error())
		}
		if res != nil {
			resources = res.Resources
		}
	}
	for _, tenantID := range ids {
		var limitMemory int64
		if l, ok := limits[tenantID]; ok && l != 0 {
			limitMemory = int64(l)
		} else {
			limitMemory = clusterStats.AllMemory
		}
		result[tenantID] = map[string]interface{}{
			"tenant_id":           tenantID,
			"limit_memory":        limitMemory,
			"limit_cpu":           clusterStats.AllCPU,
			"service_total_num":   serviceTenantCount[tenantID],
			"disk":                0,
			"service_running_num": 0,
			"cpu":                 0,
			"memory":              0,
		}
		tr, _ := resources[tenantID]
		if tr != nil {
			result[tenantID]["service_running_num"] = tr.RunningAppNum
			result[tenantID]["cpu"] = tr.CpuRequest
			result[tenantID]["memory"] = tr.MemoryRequest
		}
	}
	//query disk used in prometheus
	query := fmt.Sprintf(`sum(app_resource_appfs{tenant_id=~"%s"}) by(tenant_id)`, strings.Join(ids, "|"))
	metric := t.prometheusCli.GetMetric(query, time.Now())
	for _, mv := range metric.MetricData.MetricValues {
		var tenantID = mv.Metadata["tenant_id"]
		var disk int
		if mv.Sample != nil {
			disk = int(mv.Sample.Value() / 1024)
		}
		if tenantID != "" {
			result[tenantID]["disk"] = disk / 1024
		}
	}
	return result, nil
}

//TenantResourceStats tenant resource stats
type TenantResourceStats struct {
	TenantID         string `json:"tenant_id,omitempty"`
	CPURequest       int64  `json:"cpu_request,omitempty"`
	CPULimit         int64  `json:"cpu_limit,omitempty"`
	MemoryRequest    int64  `json:"memory_request,omitempty"`
	MemoryLimit      int64  `json:"memory_limit,omitempty"`
	RunningAppNum    int64  `json:"running_app_num"`
	UnscdCPUReq      int64  `json:"unscd_cpu_req,omitempty"`
	UnscdCPULimit    int64  `json:"unscd_cpu_limit,omitempty"`
	UnscdMemoryReq   int64  `json:"unscd_memory_req,omitempty"`
	UnscdMemoryLimit int64  `json:"unscd_memory_limit,omitempty"`
}

//GetTenantResource get tenant resource
func (t *TenantAction) GetTenantResource(tenantID string) (ts TenantResourceStats, err error) {
	tr, err := t.statusCli.GetTenantResource(tenantID)
	if err != nil {
		return ts, err
	}
	ts.TenantID = tenantID
	ts.CPULimit = tr.CpuLimit
	ts.CPURequest = tr.CpuRequest
	ts.MemoryLimit = tr.MemoryLimit
	ts.MemoryRequest = tr.MemoryRequest
	ts.RunningAppNum = tr.RunningAppNum
	return
}

//ClusterResourceStats cluster resource stats
type ClusterResourceStats struct {
	AllCPU        int64
	AllMemory     int64
	RequestCPU    int64
	RequestMemory int64
}

func (t *TenantAction) initClusterResource(ctx context.Context) error {
	if t.cacheClusterResourceStats == nil || t.cacheTime.Add(time.Minute*3).Before(time.Now()) {
		var crs ClusterResourceStats
		nodes, err := t.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("get cluster nodes failure %s", err.Error())
			return err
		}
		for _, node := range nodes.Items {
			// check if node contains taints
			if containsTaints(&node) {
				logrus.Debugf("[GetClusterInfo] node(%s) contains NoSchedule taints", node.GetName())
				continue
			}
			if node.Spec.Unschedulable {
				continue
			}
			for _, c := range node.Status.Conditions {
				if c.Type == v1.NodeReady && c.Status != v1.ConditionTrue {
					continue
				}
			}
			crs.AllMemory += node.Status.Allocatable.Memory().Value() / (1024 * 1024)
			crs.AllCPU += node.Status.Allocatable.Cpu().MilliValue()
		}
		t.cacheClusterResourceStats = &crs
		t.cacheTime = time.Now()
	}
	return nil
}

// GetAllocatableResources returns allocatable cpu and memory (MB)
func (t *TenantAction) GetAllocatableResources(ctx context.Context) (*ClusterResourceStats, error) {
	var crs ClusterResourceStats
	if t.initClusterResource(ctx) != nil {
		return &crs, nil
	}
	ts, err := t.statusCli.GetAllTenantResource()
	if err != nil {
		logrus.Errorf("get tenant resource failure %s", err.Error())
	}
	re := t.cacheClusterResourceStats
	if ts != nil {
		crs.RequestCPU = 0
		crs.RequestMemory = 0
		for _, re := range ts.Resources {
			crs.RequestCPU += re.CpuRequest
			crs.RequestMemory += re.MemoryRequest
		}
	}
	return re, nil
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

	podList, err := t.statusCli.GetMultiServicePods(running)
	if err != nil {
		return nil, err
	}

	res := make(map[string]map[string]interface{})
	for serviceID, item := range podList.ServicePods {
		pods := item.NewPods
		pods = append(pods, item.OldPods...)
		var memory, cpu int64
		for _, pod := range pods {
			for _, c := range pod.Containers {
				memory += c.MemoryLimit
				cpu += c.CpuRequest
			}
		}
		res[serviceID] = map[string]interface{}{"memory": memory / 1024 / 1024, "cpu": cpu}
	}

	for _, c := range closed {
		res[c] = map[string]interface{}{"memory": 0, "cpu": 0}
	}

	disks := GetServicesDiskDeprecated(tr.Body.ServiceIDs, t.prometheusCli)
	for serviceID, disk := range disks {
		if _, ok := res[serviceID]; ok {
			res[serviceID]["disk"] = disk / 1024
		} else {
			res[serviceID] = make(map[string]interface{})
			res[serviceID]["disk"] = disk / 1024
		}
	}
	return res, nil
}

func (t *TenantAction) getPodNums(serviceID string) int {
	pods, err := t.statusCli.GetAppPods(context.TODO(), &pb.ServiceRequest{
		ServiceId: serviceID,
	})

	if err != nil {
		logrus.Warningf("get app pods: %v", err)
		return 0
	}

	return len(pods.OldPods) + len(pods.NewPods)
}

//TenantsSum TenantsSum
func (t *TenantAction) TenantsSum() (int, error) {
	s, err := db.GetManager().TenantDao().GetALLTenants("")
	if err != nil {
		return 0, err
	}
	return len(s), nil
}

//GetProtocols GetProtocols
func (t *TenantAction) GetProtocols() ([]*dbmodel.RegionProcotols, *util.APIHandleError) {
	return []*dbmodel.RegionProcotols{
		{
			ProtocolGroup: "http",
			ProtocolChild: "http",
			APIVersion:    "v2",
			IsSupport:     true,
		},
		{
			ProtocolGroup: "http",
			ProtocolChild: "grpc",
			APIVersion:    "v2",
			IsSupport:     true,
		}, {
			ProtocolGroup: "stream",
			ProtocolChild: "tcp",
			APIVersion:    "v2",
			IsSupport:     true,
		}, {
			ProtocolGroup: "stream",
			ProtocolChild: "udp",
			APIVersion:    "v2",
			IsSupport:     true,
		}, {
			ProtocolGroup: "stream",
			ProtocolChild: "mysql",
			APIVersion:    "v2",
			IsSupport:     true,
		},
	}, nil
}

//TransPlugins TransPlugins
func (t *TenantAction) TransPlugins(tenantID, tenantName, fromTenant string, pluginList []string) *util.APIHandleError {
	tenantInfo, err := db.GetManager().TenantDao().GetTenantIDByName(fromTenant)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("get tenant infos", err)
	}
	goodrainID := tenantInfo.UUID
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
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

// GetServicesStatus returns a list of service status matching ids.
func (t *TenantAction) GetServicesStatus(ids string) map[string]string {
	return t.statusCli.GetStatuss(ids)
}

//IsClosedStatus checks if the status is closed status.
func (t *TenantAction) IsClosedStatus(status string) bool {
	return t.statusCli.IsClosedStatus(status)
}

//GetClusterResource get cluster resource
func (t *TenantAction) GetClusterResource(ctx context.Context) *ClusterResourceStats {
	if t.initClusterResource(ctx) != nil {
		return nil
	}
	return t.cacheClusterResourceStats
}

// CheckResourceName checks resource name.
func (t *TenantAction) CheckResourceName(ctx context.Context, namespace string, req *model.CheckResourceNameReq) (*model.CheckResourceNameResp, error) {
	obj, ok := t.resources[req.Type]
	if !ok {
		return nil, bcode.NewBadRequest("unsupported resource: " + req.Type)
	}

	nctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	retries := 3
	for i := 0; i < retries; i++ {
		if err := t.k8sClient.Get(nctx, types.NamespacedName{Namespace: namespace, Name: req.Name}, obj); err != nil {
			if k8sErrors.IsNotFound(err) {
				break
			}
			return nil, errors.Wrap(err, "ensure app name")
		}
		req.Name += "-" + rutil.NewUUID()[:5]
	}

	return &model.CheckResourceNameResp{
		Name: req.Name,
	}, nil
}
