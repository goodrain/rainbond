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
	"github.com/goodrain/rainbond/pkg/component/grpc"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/component/mq"
	"github.com/goodrain/rainbond/pkg/component/prom"
	"github.com/goodrain/rainbond/util/constants"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/fields"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goodrain/rainbond/api/client/prometheus"
	"github.com/goodrain/rainbond/api/model"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/api/util/bcode"
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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// TenantAction tenant act
type TenantAction struct {
	MQClient                  mqclient.MQClient
	statusCli                 *client.AppRuntimeSyncClient
	kubeClient                *kubernetes.Clientset
	cacheClusterResourceStats *ClusterResourceStats
	cacheTime                 time.Time
	prometheusCli             prometheus.Interface
	k8sClient                 k8sclient.Client
	resources                 map[string]k8sclient.Object
}

// CreateTenManager create Manger
func CreateTenManager() *TenantAction {
	resources := map[string]k8sclient.Object{
		"helmApp": &v1alpha1.HelmApp{},
		"service": &corev1.Service{},
	}
	return &TenantAction{
		MQClient:      mq.Default().MqClient,
		statusCli:     grpc.Default().StatusClient,
		kubeClient:    k8s.Default().Clientset,
		prometheusCli: prom.Default().PrometheusCli,
		k8sClient:     k8s.Default().K8sClient,
		resources:     resources,
	}
}

// BindTenantsResource query tenant resource used and sort
func (t *TenantAction) BindTenantsResource(source []*dbmodel.Tenants) apimodel.TenantList {
	var list apimodel.TenantList
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
		var item = &apimodel.TenantAndResource{
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
			item.RunningApplications = re.RunningApplications
		}
		list.Add(item)
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].MemoryRequest > list[j].MemoryRequest {
			return true
		}
		return false
	})
	return list
}

// GetTenants get tenants
func (t *TenantAction) GetTenants(query string) ([]*dbmodel.Tenants, error) {
	tenants, err := db.GetManager().TenantDao().GetALLTenants(query)
	if err != nil {
		return nil, err
	}
	return tenants, err
}

// GetTenantsByTenantIDs get tenants ids
func (t *TenantAction) GetTenantsByTenantIDs(TenantIDs []string) ([]*dbmodel.Tenants, error) {
	tenants, err := db.GetManager().TenantDao().GetTenantsByTenantIDs(TenantIDs)
	if err != nil {
		return nil, err
	}
	return tenants, err
}

// GetTenantsByEid GetTenantsByEid
func (t *TenantAction) GetTenantsByEid(eid, query string) ([]*dbmodel.Tenants, error) {
	tenants, err := db.GetManager().TenantDao().GetTenantByEid(eid, query)
	if err != nil {
		return nil, err
	}
	return tenants, err
}

// UpdateTenant update tenant info
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

// TotalMemCPU StatsMemCPU
func (t *TenantAction) TotalMemCPU(services []*dbmodel.TenantServices) (*apimodel.StatsInfo, error) {
	cpus := 0
	mem := 0
	for _, service := range services {
		logrus.Debugf("service is %d, cpus is %d, mem is %v", service.ID, service.ContainerCPU, service.ContainerMemory)
		cpus += service.ContainerCPU
		mem += service.ContainerMemory
	}
	si := &apimodel.StatsInfo{
		CPU: cpus,
		MEM: mem,
	}
	return si, nil
}

// GetTenantsName get tenants name
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

// GetTenantsByName get tenants
func (t *TenantAction) GetTenantsByName(name string) (*dbmodel.Tenants, error) {
	tenant, err := db.GetManager().TenantDao().GetTenantIDByName(name)
	if err != nil {
		return nil, err
	}
	return tenant, err
}

// GetTenantsByUUID get tenants
func (t *TenantAction) GetTenantsByUUID(uuid string) (*dbmodel.Tenants, error) {
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(uuid)
	if err != nil {
		return nil, err
	}

	return tenant, err
}

// StatsMemCPU StatsMemCPU
func (t *TenantAction) StatsMemCPU(services []*dbmodel.TenantServices) (*apimodel.StatsInfo, error) {
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
	si := &apimodel.StatsInfo{
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

// GetTenantsResources Gets the resource usage of the specified tenant.
func (t *TenantAction) GetTenantsResources(ctx context.Context, tr *apimodel.TenantResources) (map[string]map[string]interface{}, error) {
	//ids, err := db.GetManager().TenantDao().GetTenantIDsByNames(tr.Body.TenantNames)
	//if err != nil {
	//	return nil, err
	//}
	limits, ids, err := db.GetManager().TenantDao().GetTenantLimitsByNames(tr.Body.TenantNames)
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
		var limitMemory, limitCPU, limitStorage int64
		if l, ok := limits[tenantID]; ok && l != nil {
			limitMemory = int64(l.LimitMemory)
			limitCPU = int64(l.LimitCPU)
			limitStorage = int64(l.LimitStorage)
		} else {
			limitMemory = clusterStats.AllMemory
			limitCPU = clusterStats.AllCPU
			limitStorage = int64(clusterStats.TotalDisk)
		}
		result[tenantID] = map[string]interface{}{
			"tenant_id":           tenantID,
			"limit_memory":        limitMemory,
			"limit_cpu":           limitCPU,
			"limit_storage":       limitStorage,
			"service_total_num":   serviceTenantCount[tenantID],
			"disk":                0,
			"service_running_num": 0,
			"cpu":                 0,
			"memory":              0,
			"app_running_num":     0,
		}
		tr, _ := resources[tenantID]
		if tr != nil {
			result[tenantID]["service_running_num"] = tr.RunningAppNum
			result[tenantID]["cpu"] = tr.CpuRequest
			result[tenantID]["memory"] = tr.MemoryRequest
			result[tenantID]["app_running_num"] = tr.RunningApplications
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
			result[tenantID]["disk"] = disk
		}
	}
	return result, nil
}

// TenantResourceStats tenant resource stats
type TenantResourceStats struct {
	TenantID            string `json:"tenant_id,omitempty"`
	CPURequest          int64  `json:"cpu_request,omitempty"`
	CPULimit            int64  `json:"cpu_limit,omitempty"`
	MemoryRequest       int64  `json:"memory_request,omitempty"`
	MemoryLimit         int64  `json:"memory_limit,omitempty"`
	RunningAppNum       int64  `json:"running_app_num"`
	UnscdCPUReq         int64  `json:"unscd_cpu_req,omitempty"`
	UnscdCPULimit       int64  `json:"unscd_cpu_limit,omitempty"`
	UnscdMemoryReq      int64  `json:"unscd_memory_req,omitempty"`
	UnscdMemoryLimit    int64  `json:"unscd_memory_limit,omitempty"`
	RunningApplications int64  `json:"running_applications"`
}

// GetTenantResource get tenant resource
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
	ts.RunningApplications = tr.RunningApplications
	return
}

// PodResourceInformation -
type PodResourceInformation struct {
	NodeName         string
	ServiceID        string
	AppID            string
	Memory           int64
	ResourceVersion  string
	CPU              int64
	StorageEphemeral int64
}

// NodeGPU -
type NodeGPU struct {
	NodeName string
	GPUCount int64
	GPUMem   int64
}

// ClusterResourceStats cluster resource stats
type ClusterResourceStats struct {
	AllCPU        int64
	AllMemory     int64
	AllGPU        int64
	NodeGPU       []NodeGPU
	RequestCPU    int64
	RequestMemory int64
	UsageDisk     uint64
	TotalDisk     uint64
	NodePods      []PodResourceInformation
	AllPods       int64
}

func (t *TenantAction) initClusterResource(ctx context.Context) error {
	if t.cacheClusterResourceStats == nil || t.cacheTime.Add(time.Minute*3).Before(time.Now()) {
		var crs ClusterResourceStats
		nodes, err := t.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("get cluster nodes failure %s", err.Error())
			return err
		}
		usedNodeList := make([]v1.Node, len(nodes.Items))
		for i, node := range nodes.Items {
			// check if node contains taints
			if containsTaints(&node) {
				logrus.Debugf("[GetClusterInfo] node(%s) contains NoSchedule taints", node.GetName())
				continue
			}
			usedNodeList[i] = node
			for _, c := range node.Status.Conditions {
				if c.Type == v1.NodeReady && c.Status != v1.ConditionTrue {
					continue
				}
			}
			crs.AllMemory += node.Status.Allocatable.Memory().Value() / (1024 * 1024)
			crs.AllCPU += node.Status.Allocatable.Cpu().MilliValue()
		}
		var nodePodsList []PodResourceInformation
		for i := range usedNodeList {
			node := usedNodeList[i]
			time.Sleep(50 * time.Microsecond)
			labelSelector := "creator=Rainbond"
			podList, err := t.kubeClient.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
				FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node.Name}).String(),
				LabelSelector: labelSelector,
			})
			if err != nil {
				logrus.Errorf("get node %v pods error:%v", node.Name, err)
				continue
			}
			crs.AllPods += int64(len(podList.Items))
			for _, pod := range podList.Items {
				var nodePod PodResourceInformation
				nodePod.NodeName = node.Name
				if componentID, ok := pod.Labels["service_id"]; ok {
					nodePod.ServiceID = componentID
				}
				if appID, ok := pod.Labels["app_id"]; ok {
					nodePod.AppID = appID
				}
				nodePod.ResourceVersion = pod.ResourceVersion
				for _, c := range pod.Spec.Containers {
					nodePod.Memory += c.Resources.Requests.Memory().Value()
					nodePod.CPU += c.Resources.Requests.Cpu().MilliValue()
					nodePod.StorageEphemeral += c.Resources.Requests.StorageEphemeral().Value()
				}
				nodePodsList = append(nodePodsList, nodePod)
			}
		}
		crs.NodePods = nodePodsList
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

// GetServicesResources Gets the resource usage of the specified service.
func (t *TenantAction) GetServicesResources(tr *apimodel.ServicesResources) (re map[string]map[string]interface{}, err error) {
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
				memory += c.MemoryRequest
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

// TenantsSum TenantsSum
func (t *TenantAction) TenantsSum() (int, error) {
	s, err := db.GetManager().TenantDao().GetALLTenants("")
	if err != nil {
		return 0, err
	}
	return len(s), nil
}

// GetProtocols GetProtocols
func (t *TenantAction) GetProtocols() ([]*dbmodel.RegionProcotols, *util.APIHandleError) {
	return []*dbmodel.RegionProcotols{
		{
			ProtocolGroup: "http",
			ProtocolChild: "http",
			APIVersion:    "v2",
			IsSupport:     true,
		},
		{
			ProtocolGroup: "stream",
			ProtocolChild: "tcp",
			APIVersion:    "v2",
			IsSupport:     true,
		}, {
			ProtocolGroup: "stream",
			ProtocolChild: "udp",
			APIVersion:    "v2",
			IsSupport:     true,
		},
	}, nil
}

// TransPlugins TransPlugins
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

// IsClosedStatus checks if the status is closed status.
func (t *TenantAction) IsClosedStatus(status string) bool {
	return t.statusCli.IsClosedStatus(status)
}

// GetClusterResource get cluster resource
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

// TenantResourceQuota - 设置或更新租户的资源配额和限制范围。
func (t *TenantAction) TenantResourceQuota(ctx context.Context, namespace string, limitCPU, limitMemory, LimitStorage int) error {
	// ConvertMemory, ConvertCPU, ConvertStorage 函数将 limit 值转换为字符串表示
	memory := ConvertMemory(limitMemory)
	cpu := ConvertCPU(limitCPU)
	storage := ConvertStorage(LimitStorage)
	// 创建一个K8s的资源对象，用于存储资源配额的信息
	resources := make(map[corev1.ResourceName]resource.Quantity)
	if limitCPU != 0 {
		resources[corev1.ResourceLimitsCPU] = resource.MustParse(cpu)
	}
	if limitMemory != 0 {
		resources[corev1.ResourceLimitsMemory] = resource.MustParse(memory)
	}
	if LimitStorage != 0 {
		resources[corev1.ResourceRequestsStorage] = resource.MustParse(storage)
	}

	// ResourceQuota 对象定义了在命名空间内的资源配额。
	// 该配额限制了命名空间中可用的总资源量。
	// 详细内容参考：https://kubernetes.io/zh-cn/docs/concepts/policy/resource-quotas/
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-%v", namespace, "limits-quota"),
			Namespace: namespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: resources,
		},
	}

	// 尝试在 Kubernetes 中创建 ResourceQuota 对象。
	_, err := t.kubeClient.CoreV1().ResourceQuotas(namespace).Create(ctx, quota, metav1.CreateOptions{})
	if err != nil {
		// 如果配额已存在，则更新或删除配额。
		if k8sErrors.IsAlreadyExists(err) {
			if limitCPU == 0 && limitMemory == 0 && LimitStorage == 0 {
				// 如果所有限制为零，则删除配额。
				err = t.kubeClient.CoreV1().ResourceQuotas(namespace).Delete(ctx, fmt.Sprintf("%v-limits-quota", namespace), metav1.DeleteOptions{})
				if err != nil {
					return errors.Wrap(err, "delete tenant quotas failure")
				}
			} else {
				// 否则，更新现有的配额。
				_, err = t.kubeClient.CoreV1().ResourceQuotas(namespace).Update(ctx, quota, metav1.UpdateOptions{})
				if err != nil {
					return errors.Wrap(err, "update tenant quotas failure")
				}
			}
		} else {
			return err
		}
	}

	// LimitRangeItem 对象定义了在命名空间内单个容器或 Pod 可使用的资源范围。
	// 这里设置了 CPU 和内存的默认限制。
	// 详细内容参考：https://kubernetes.io/zh-cn/docs/concepts/policy/limit-range/
	var lrs []v1.LimitRangeItem
	lrsResource := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("128m"),
		corev1.ResourceMemory: resource.MustParse("512Mi"),
	}
	lrs = append(lrs, v1.LimitRangeItem{
		Default:        lrsResource,
		DefaultRequest: lrsResource,
		Type:           corev1.LimitTypeContainer,
	})

	// 初始化 LimitRange 对象，它可以强制执行容器或 Pod 级别的资源限制。
	lr := &v1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-%v", namespace, "limits-range"),
			Namespace: namespace,
		},
		Spec: v1.LimitRangeSpec{
			Limits: lrs,
		},
	}

	// 尝试在 Kubernetes 中创建 LimitRange 对象。
	_, err = t.kubeClient.CoreV1().LimitRanges(namespace).Create(ctx, lr, metav1.CreateOptions{})
	if err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			if limitCPU == 0 && limitMemory == 0 {
				// 如果 LimitRange 已存在且不限制内存和CPU的情况下，则删除 LimitRange。
				err = t.kubeClient.CoreV1().LimitRanges(namespace).Delete(ctx, fmt.Sprintf("%v-limits-range", namespace), metav1.DeleteOptions{})
				if err != nil {
					return errors.Wrap(err, "delete tenant limit range failure")
				}
			}
		} else {
			return err
		}
	}
	return nil
}

func ConvertMemory(memory int) string {
	if memory >= 1024 {
		return fmt.Sprintf("%vGi", memory/1024)
	}
	return fmt.Sprintf("%vMi", memory)
}

func ConvertCPU(cpu int) string {
	if cpu >= 1000 {
		return fmt.Sprintf("%v", strconv.Itoa(cpu/1000))
	}
	return fmt.Sprintf("%vm", cpu)
}

func ConvertStorage(storage int) string {
	return fmt.Sprintf("%vGi", strconv.Itoa(storage))
}

func (t *TenantAction) CheckTenantResourceQuotaAndLimitRange(ctx context.Context, namespace string, noMemory, noCPU int) error {
	quotas, err := t.kubeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, fmt.Sprintf("%v-%v", namespace, "limits-quota"), metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), constants.NotFound) {
			return nil
		}
		return errors.Wrap(err, "get tenant limit range failure")
	}
	hardCpu := quotas.Status.Hard["limits.cpu"]
	userCpu := quotas.Status.Used["limits.cpu"]
	hardMemory := quotas.Status.Hard["limits.memory"]
	userMemory := quotas.Status.Used["limits.memory"]

	surplusCPU := ConvertCpuToInt(hardCpu.String()) - ConvertCpuToInt(userCpu.String())
	surplusMemory := hardMemory.Value() - userMemory.Value()
	limitRanges, err := t.kubeClient.CoreV1().LimitRanges(namespace).Get(ctx, fmt.Sprintf("%v-%v", namespace, "limits-range"), metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), constants.NotFound) {
			return nil
		}
		return errors.Wrap(err, "get tenant limit range failure")
	}
	var defaultCpu, defaultMemory int64
	for _, limit := range limitRanges.Spec.Limits {
		cpu := limit.Default["cpu"]
		defaultCpu = ConvertCpuToInt(cpu.String())
		defaultMemory = limit.Default.Memory().Value()
	}
	if ConvertCpuToInt(hardCpu.String()) > 0 && int64(noCPU)*defaultCpu > surplusCPU {
		return errors.New(constants.TenantQuotaCPULack)
	}
	if hardMemory.Value() > 0 && int64(noMemory)*defaultMemory > surplusMemory {
		return errors.New(constants.TenantQuotaMemoryLack)
	}
	return nil
}

func ConvertCpuToInt(cpu string) int64 {
	var res int64
	if strings.Contains(cpu, "m") {
		s := strings.TrimRight(cpu, "m")
		res, _ = strconv.ParseInt(s, 10, 64)
		return res
	}
	res, _ = strconv.ParseInt(cpu, 10, 64)
	return res * 1000
}
