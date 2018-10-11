package clean

import (
	"container/list"
	"context"
	"os"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"github.com/jinzhu/gorm"
)

//Resource should be clean resource
type Resource interface {
	IsTimeout() bool
	DeleteResources() error
	IsClean() bool
	Name() string
	Type() string
}

type nameSpacesResource struct {
	manager    *Manager
	namespaces string
	id         string
	createTime time.Time
}

type tenantServiceResource struct {
	manager    *Manager
	serviceId  string
	createTime time.Time
	query      *model.TenantServicesDelete
}

type rcResource struct {
	manager    *Manager
	id         string
	namespaces string
	createTime time.Time
	serviceId string
}
type statefulResource struct {
	manager    *Manager
	id         string
	namespaces string
	createTime time.Time
	serviceId string
}
type deploymentResource struct {
	manager    *Manager
	id         string
	namespaces string
	createTime time.Time
	serviceId string
}

type k8sServiceResource struct {
	manager    *Manager
	id         string
	namespaces string
	createTime time.Time
}

type servicePodResource struct {
	manager    *Manager
	podName    string
	createTime time.Time
}

func (p *servicePodResource) IsTimeout() bool {
	return true
}

func (p *servicePodResource) DeleteResources() error {
	if err := db.GetManager().K8sPodDao().DeleteK8sPodByName(p.podName); err != nil {
		return err
	}
	logrus.Info("delete servicePodResource success：", p.podName)
	return nil
}

func (p *servicePodResource) IsClean() bool {
	return true

}

func (p *servicePodResource) Name() string {
	return p.podName
}

func (p *servicePodResource) Type() string {
	return "ServicePod"
}

func QueryPodsResource(m *Manager) []Resource {
	podsNameList := make([]string, 0, 100)
	k8sPodsList := make([]Resource, 0, 100)
	podsList, err := m.kubeclient.Pods(v1.NamespaceAll).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Error(err.Error())
	}

	for _, v := range podsList.Items {
		podsNameList = append(podsNameList, v.Name)
	}
	val, err := db.GetManager().K8sPodDao().GetK8sPodByNotInPodNameList(podsNameList)
	if err != nil {
		logrus.Error(err)
	}
	for _, v := range val {
		s := &servicePodResource{
			podName:    v.PodName,
			createTime: time.Now(),
			manager:    m,
		}
		k8sPodsList = append(k8sPodsList, s)
	}
	return k8sPodsList
}

func (t *tenantServiceResource) IsTimeout() bool {
	return true

}
func (t *tenantServiceResource) DeleteResources() error {
	versionInfoList, err := db.GetManager().VersionInfoDao().GetVersionByServiceID(t.serviceId)
	if err != nil {
		return err
	}
	for _, v := range versionInfoList {
		if v.FinalStatus == "success" {
			if v.DeliveredType == "slug" {
				if err := os.RemoveAll(v.DeliveredPath); err != nil {
					if !strings.Contains(err.Error(), "no such file or directory") {
						return err
					}
				}
				logrus.Info("Clean up deleted application build resources file successfully:", v.DeliveredPath)

				if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(v); err != nil {
					return err
				}
			}

		} else {
			if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(v); err != nil {
				return err
			}
		}

	}

	EventList, err := db.GetManager().ServiceEventDao().GetEventByServiceID(t.serviceId)
	if err != nil {
		return err
	}

	for _, v := range EventList {
		err := db.GetManager().EventLogDao().DeleteServiceEventLogByEventId(v.EventID)
		if err != nil {
			return err
		}
		if err := db.GetManager().ServiceEventDao().DelEventByServiceID(t.serviceId); err != nil {
			return err
		}
	}

	if err := db.GetManager().TenantServiceDeleteDao().DeleteTenantServicesDelete(t.query); err != nil {
		return err
	}
	logrus.Info("Application related data clean up successfully,serviceID:", t.serviceId)
	return nil
}

func (t *tenantServiceResource) IsClean() bool {
	return true
}

func (t *tenantServiceResource) Name() string {
	return t.serviceId
}

func (t *tenantServiceResource) Type() string {
	return "tenantService"
}

func QueryTenantServiceResource(m *Manager) []Resource {
	TenantServiceList := make([]Resource, 0, 100)
	now := time.Now()
	create_time := now.AddDate(0, 0, -7)
	serviceDelList, err := db.GetManager().TenantServiceDeleteDao().GetTenantServicesDeleteByCreateTime(create_time)
	if err != nil {
		logrus.Error(err)
	} else {

	}
	for _, v := range serviceDelList {
		s := &tenantServiceResource{
			serviceId:  v.ServiceID,
			createTime: time.Now(),
			query:      v,
			manager:    m,
		}
		TenantServiceList = append(TenantServiceList, s)
	}
	return TenantServiceList
}

func (k *k8sServiceResource) IsTimeout() bool {
	now := time.Now()
	if now.After(k.createTime.Add(time.Minute * 5)) {
		return true
	}
	return false

}

func (k *k8sServiceResource) DeleteResources() error {
	if err := k.manager.kubeclient.CoreV1().Services(k.namespaces).Delete(k.id, &meta_v1.DeleteOptions{}); err != nil {
		return err
	} else {
		logrus.Info("delete k8sServiceResource success：", k.id)
		return nil
	}
}

func (k *k8sServiceResource) IsClean() bool {
	isNotExist := db.GetManager().K8sServiceDao().K8sServiceIsExist(k.namespaces, k.id)

	if isNotExist {
		return true
	} else {
		return false
	}
}

func (k *k8sServiceResource) Name() string {
	return k.id
}

func (k *k8sServiceResource) Type() string {
	return "k8sService"
}

func QueryK8sServiceResource(m *Manager) []Resource {
	ServivesMap := make(map[string][]string)
	serviceList := make([]Resource, 0, 100)

	services, err := db.GetManager().K8sServiceDao().GetAllK8sService()
	if err != nil {
		logrus.Error(err)
	}

	for _, v := range services {

		if _, ok := ServivesMap[v.TenantID]; ok {
			ServivesMap[v.TenantID] = append(ServivesMap[v.TenantID], v.K8sServiceID)
		} else {
			ServivesMap[v.TenantID] = []string{v.K8sServiceID}
		}

	}

	ServicesList, err := m.kubeclient.Services(v1.NamespaceAll).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Error(err)
	}
	for _, v := range ServicesList.Items {
		val, ok := ServivesMap[v.Namespace]
		if ok {
			if !InSlice(v.Name, val) || isRainBondResource(v.Labels) {
				s := &k8sServiceResource{
					manager:    m,
					createTime: time.Now(),
					namespaces: v.Namespace,
					id:         v.Name,
				}
				serviceList = append(serviceList, s)
			}
		}

	}
	return serviceList
}

func (d *deploymentResource) IsTimeout() bool {
	now := time.Now()
	if now.After(d.createTime.Add(time.Minute * 5)) {
		return true
	}
	return false

}

func (d *deploymentResource) DeleteResources() error {
	pods, err := db.GetManager().K8sPodDao().GetPodByService(d.serviceId)
	if err != nil {
		logrus.Error("delete deploymentResource GetPodByService error.", err.Error())
	}
	if pods != nil && len(pods) > 0 {
		for i := range pods {
			pod := pods[i]
			err = d.manager.kubeclient.CoreV1().Pods(d.namespaces).Delete(pod.PodName, &meta_v1.DeleteOptions{})
			if err != nil {
				if err = checkNotFoundError(err); err != nil {
					logrus.Errorf("delete pod (%s) from k8s api error %s", pod.PodName, err.Error())
				}
			}

		}
		err = db.GetManager().K8sPodDao().DeleteK8sPod(d.serviceId)
		if err != nil {
			if err != gorm.ErrRecordNotFound {
				logrus.Error("delete pods by service id error.", err.Error())
			}
		}
	}
	if err := d.manager.kubeclient.AppsV1beta1().Deployments(d.namespaces).Delete(d.id, &meta_v1.DeleteOptions{}); err != nil {
		if err = checkNotFoundError(err); err != nil {
			return err
		}
	}
	logrus.Info("delete deployment success：", d.id)
	return nil
}

func (d *deploymentResource) IsClean() bool {
	isNotExist := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationIsExist(d.namespaces, "statefulset", d.id, false)

	if isNotExist {
		return true
	} else {
		return false
	}
}

func (d *deploymentResource) Name() string {
	return d.id
}

func (d *deploymentResource) Type() string {
	return "deployment"
}

func QueryDeploymentResource(m *Manager) []Resource {
	DeploymentMap := make(map[string][]string)
	DeploymentDelList := make([]Resource, 0, 100)
	DeleteList, err := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationByIsDelete("deployment", true)
	if err != nil {
		logrus.Error(err)
	}

	for _, v := range DeleteList {

		if _, ok := DeploymentMap[v.TenantID]; ok {
			DeploymentMap[v.TenantID] = append(DeploymentMap[v.TenantID], v.ReplicationID)
		} else {
			DeploymentMap[v.TenantID] = []string{v.ReplicationID}
		}

	}

	DeploymentList, err := m.kubeclient.AppsV1beta1().Deployments(v1.NamespaceAll).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Error(err)
	}
	for _, v := range DeploymentList.Items {
		val, ok := DeploymentMap[v.Namespace]
		if ok {
			if InSlice(v.Name, val) || isRainBondResource(v.Labels) {
				s := &deploymentResource{
					manager:    m,
					createTime: time.Now(),
					namespaces: v.Namespace,
					id:         v.Name,
					serviceId:getServiceId(v.Labels),
				}
				DeploymentDelList = append(DeploymentDelList, s)
			}
		}

	}
	return DeploymentDelList
}

func (s *statefulResource) IsTimeout() bool {
	now := time.Now()
	if now.After(s.createTime.Add(time.Minute * 5)) {
		return true
	}
	return false

}

func (s *statefulResource) DeleteResources() error {
	pods, err := db.GetManager().K8sPodDao().GetPodByService(s.serviceId)
	if err != nil {
		logrus.Error("delete statefulResource GetPodByService error.", err.Error())
	}
	if pods != nil && len(pods) > 0 {
		for i := range pods {
			pod := pods[i]
			err = s.manager.kubeclient.CoreV1().Pods(s.namespaces).Delete(pod.PodName, &meta_v1.DeleteOptions{})
			if err != nil {
				if err = checkNotFoundError(err); err != nil {
					logrus.Errorf("delete pod (%s) from k8s api error %s", pod.PodName, err.Error())
				}
			}
		}
		err = db.GetManager().K8sPodDao().DeleteK8sPod(s.serviceId)
		if err != nil {
			if err != gorm.ErrRecordNotFound {
				logrus.Error("delete pods by service id error.", err.Error())
			}
		}
	}
	if err := s.manager.kubeclient.AppsV1beta1().StatefulSets(s.namespaces).Delete(s.id, &meta_v1.DeleteOptions{}); err != nil {
		if err = checkNotFoundError(err); err != nil {
			return err
		}
	}
	logrus.Info("delete statefulset success：", s.id)
	return nil

}

func (s *statefulResource) IsClean() bool {
	isNotExist := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationIsExist(s.namespaces, "statefulset", s.id, false)

	if isNotExist {
		return true
	} else {
		return false
	}
}

func (s *statefulResource) Name() string {
	return s.id
}

func (s *statefulResource) Type() string {
	return "statefulset"
}

func QueryStatefulResource(m *Manager) []Resource {
	StatefulSetsMap := make(map[string][]string)
	StatefulSetList := make([]Resource, 0, 100)
	DeleteList, err := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationByIsDelete("statefulset", true)
	if err != nil {
		logrus.Error(err)
	}

	for _, v := range DeleteList {

		if _, ok := StatefulSetsMap[v.TenantID]; ok {
			StatefulSetsMap[v.TenantID] = append(StatefulSetsMap[v.TenantID], v.ReplicationID)
		} else {
			StatefulSetsMap[v.TenantID] = []string{v.ReplicationID}
		}

	}

	StatefulSetsList, err := m.kubeclient.StatefulSets(v1.NamespaceAll).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Error(err)
	}
	for _, v := range StatefulSetsList.Items {
		val, ok := StatefulSetsMap[v.Namespace]
		if ok {
			if InSlice(v.Name, val) || isRainBondResource(v.Labels) {
				s := &statefulResource{
					manager:    m,
					createTime: time.Now(),
					namespaces: v.Namespace,
					id:         v.Name,
					serviceId:getServiceId(v.Labels),
				}
				StatefulSetList = append(StatefulSetList, s)
			}
		}

	}
	return StatefulSetList
}

func (n *nameSpacesResource) IsTimeout() bool {
	now := time.Now()
	if now.After(n.createTime.Add(time.Minute * 5)) {
		return true
	}
	return false
}

func (n *nameSpacesResource) DeleteResources() error {
	if err := n.manager.kubeclient.Namespaces().Delete(n.namespaces, &meta_v1.DeleteOptions{}); err != nil {
		return err
	} else {
		logrus.Info("delete namespaces success：", n.namespaces)
		return nil
	}
	return nil
}

func (n *nameSpacesResource) IsClean() bool {
	isNotExist := db.GetManager().TenantDao().GetTenantByUUIDIsExist(n.namespaces)
	if isNotExist {
		return true
	}
	return false
}

func (n *nameSpacesResource) Name() string {
	return n.id
}

func (n *nameSpacesResource) Type() string {
	return "namespaces"
}

func QueryNameSpacesResource(m *Manager) []Resource {
	nameList := make([]string, 0, 200)
	allList := make([]string, 0, 300)
	NamespacesList := make([]Resource, 0, 100)
	Namespaces, err := m.kubeclient.CoreV1().Namespaces().List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Error(err)
	}

	for _, v := range Namespaces.Items {
		if len(v.Name) != 32 {
			continue
		}
		nameList = append(nameList, v.Name)
	}

	AllTenantsList, err := db.GetManager().TenantDao().GetALLTenants()
	if err != nil {
		logrus.Error(err)
	}

	for _, v := range AllTenantsList {
		allList = append(allList, v.UUID)
	}

	diffList := SliceDiff(nameList, allList)
	for _, v := range diffList {
		s := &nameSpacesResource{
			manager:    m,
			createTime: time.Now(),
			id:         v,
			namespaces: v,
		}
		NamespacesList = append(NamespacesList, s)

	}
	return NamespacesList
}

func (r *rcResource) IsTimeout() bool {
	now := time.Now()
	if now.After(r.createTime.Add(time.Minute * 5)) {
		return true
	}
	return false
}

func (r *rcResource) DeleteResources() error {
	pods, err := db.GetManager().K8sPodDao().GetPodByService(r.serviceId)
	if err != nil {
		logrus.Error("delete rcResource GetPodByService error.", err.Error())
	}
	if pods != nil && len(pods) > 0 {
		for i := range pods {
			pod := pods[i]
			err = r.manager.kubeclient.CoreV1().Pods(r.namespaces).Delete(pod.PodName, &meta_v1.DeleteOptions{})
			if err != nil {
				if err = checkNotFoundError(err); err != nil {
					logrus.Errorf("delete pod (%s) from k8s api error %s", pod.PodName, err.Error())
				}
			}

		}
		err = db.GetManager().K8sPodDao().DeleteK8sPod(r.serviceId)
		if err != nil {
			if err != gorm.ErrRecordNotFound {
				logrus.Error("delete pods by service id error.", err.Error())
			}
		}
	}
	if err := r.manager.kubeclient.Core().ReplicationControllers(r.namespaces).Delete(r.id, &meta_v1.DeleteOptions{}); err != nil {
		if err = checkNotFoundError(err); err != nil {
			return err
		}
	}
	logrus.Info("delete replicationcontroller success：", r.id)
	return nil

}

func (r *rcResource) IsClean() bool {
	isNotExist := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationIsExist(r.namespaces, "replicationcontroller", r.id, false)

	if isNotExist {
		return true
	} else {
		return false
	}
}

func (r *rcResource) Name() string {
	return r.id
}

func (r *rcResource) Type() string {
	return "replicationcontroller"
}

func QueryRcResource(m *Manager) []Resource {
	ReplicationControllersMap := make(map[string][]string)
	RcList := make([]Resource, 0, 100)
	DeleteList, err := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationByIsDelete("replicationcontroller", true)
	if err != nil {
		logrus.Error(err)
	}

	for _, v := range DeleteList {
		if _, ok := ReplicationControllersMap[v.TenantID]; ok {
			ReplicationControllersMap[v.TenantID] = append(ReplicationControllersMap[v.TenantID], v.ReplicationID)
		} else {
			ReplicationControllersMap[v.TenantID] = []string{v.ReplicationID}
		}
	}

	ReplicationControllersList, err := m.kubeclient.ReplicationControllers(v1.NamespaceAll).List(meta_v1.ListOptions{})
	if err != nil {
		logrus.Error(err)
	}
	for _, v := range ReplicationControllersList.Items {
		val, ok := ReplicationControllersMap[v.Namespace]
		if ok {

			if InSlice(v.Name, val) || isRainBondResource(v.Labels) {
				s := &rcResource{
					manager:    m,
					namespaces: v.Namespace,
					id:         v.Name,
					createTime: time.Now(),
					serviceId:getServiceId(v.Labels),
				}
				RcList = append(RcList, s)
			}
		}
	}
	return RcList
}

type Manager struct {
	ctx           context.Context
	kubeclient    *kubernetes.Clientset
	waiting       []Resource
	queryResource []func(*Manager) []Resource
	l             list.List
	dclient       *client.Client
}

func NewManager(ctx context.Context, kubeclient *kubernetes.Clientset) (*Manager, error) {
	m := &Manager{
		ctx:        ctx,
		kubeclient: kubeclient,
	}
	queryResource := []func(*Manager) []Resource{
		QueryRcResource,
		QueryNameSpacesResource,
		QueryStatefulResource,
		QueryDeploymentResource,
		QueryK8sServiceResource,
		QueryTenantServiceResource,
		QueryPodsResource,
	}
	m.queryResource = queryResource
	dclient, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	m.dclient = dclient

	return m, nil
}

// InSlice checks given string in string slice or not.
func InSlice(v string, sl []string) bool {
	for _, vv := range sl {
		if vv == v {
			return true
		}
	}
	return false
}

// SliceDiff returns diff slice of slice1 - slice2.
func SliceDiff(slice1, slice2 []string) (diffSlice []string) {
	for _, v := range slice1 {
		if !InSlice(v, slice2) {
			diffSlice = append(diffSlice, v)
		}
	}
	return
}

func (m *Manager) CollectingTasks() {

	util.Exec(m.ctx, func() error {
		for _, query := range m.queryResource {
			resources := query(m)
			for _, rs := range resources {
				if rs.IsClean() {
					m.l.PushBack(rs)
				}
			}
		}
		return nil
	}, time.Minute*5)

}

func (m *Manager) PerformTasks() {

	util.Exec(m.ctx, func() error {
		for m.l.Len() > 1 {
			rs := m.l.Back()
			if res, ok := rs.Value.(Resource); ok {
				if res.IsTimeout() {
					if res.IsClean() {
						if err := res.DeleteResources(); err != nil {
							if !strings.Contains(err.Error(), "not found") {
								logrus.Error("failed to delete:", err)
							}

						}
					}
					m.l.Remove(rs)
				}
			} else {
				logrus.Error("Type conversion failed")
			}
		}
		return nil
	}, time.Minute*3)
}

func isRainBondResource(labels map[string]string) bool {
	val, ok := labels["creator"]
	if ok {
		if val == "RainBond" {
			return true
		}
		return false
	}
	return false
}

func getServiceId(labels map[string]string) string {
	val, ok := labels["service_id"]
	if ok {
		return val
	}
	return ""
}

func checkNotFoundError(err error) error {
	if strings.HasSuffix(err.Error(), "not found") {
		return nil
	}
	return err
}

func (m *Manager) Start() error {
	logrus.Info("clean up module starts....")
	go m.CollectingTasks()
	go m.PerformTasks()
	return nil

}
func (m *Manager) Stop() error {
	logrus.Info("CleanResource is stoping.")
	return nil
}
