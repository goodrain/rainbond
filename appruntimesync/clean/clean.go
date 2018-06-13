package clean

import (
	"k8s.io/client-go/kubernetes"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/db"
	"time"
	"github.com/goodrain/rainbond/util"
	"context"
	"container/list"
	"k8s.io/client-go/pkg/api/v1"
)

//Resource should be clean resource
type Resource interface {
	IsTimeout() bool
	DeleteResources() error
	IsClean() bool
	Name() string
	Type() string
}

type tenantServiceResource struct {
	manager    *Manager
	id         string
	namespaces string
	createTime time.Time
}

type rcResource struct {
	manager    *Manager
	id         string
	namespaces string
	createTime time.Time
}
type statefulResource struct {
	manager    *Manager
	id         string
	namespaces string
	createTime time.Time
}
type deploymentResource struct {
	manager    *Manager
	id         string
	namespaces string
	createTime time.Time
}

type k8sServiceResource struct {
	manager    *Manager
	id         string
	namespaces string
	createTime time.Time
}

func (k *k8sServiceResource) IsTimeout() bool {
	now := time.Now()
	if now.After(k.createTime.Add(time.Minute * 5)) {
		return true
	}
	return false

}

func (k *k8sServiceResource) DeleteResources() error {
	if err := k.manager.kubeclient.Services(k.namespaces).Delete(k.id, &meta_v1.DeleteOptions{}); err != nil {
		logrus.Error(err)
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

func queryK8sServiceResource(m *Manager) []Resource {
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
			if !InSlice(v.Name, val) {
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
	if err := d.manager.kubeclient.AppsV1beta1().Deployments(d.namespaces).Delete(d.id, &meta_v1.DeleteOptions{}); err != nil {
		logrus.Error(err)
		return err
	} else {
		logrus.Info("delete deployment success：", d.id)
		return nil
	}
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

func queryDeploymentResource(m *Manager) []Resource {
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
			if InSlice(v.Name, val) {
				s := &deploymentResource{
					manager:    m,
					createTime: time.Now(),
					namespaces: v.Namespace,
					id:         v.Name,
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

	if err := s.manager.kubeclient.StatefulSets(s.namespaces).Delete(s.id, &meta_v1.DeleteOptions{}); err != nil {
		logrus.Error(err)
		return err
	} else {
		logrus.Info("delete statefulset success：", s.id)
		return nil
	}
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

func queryStatefulResource(m *Manager) []Resource {
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
			if InSlice(v.Name, val) {
				s := &statefulResource{
					manager:    m,
					createTime: time.Now(),
					namespaces: v.Namespace,
					id:         v.Name,
				}
				StatefulSetList = append(StatefulSetList, s)
			}
		}

	}

	return StatefulSetList
}

func (t *tenantServiceResource) IsTimeout() bool {
	now := time.Now()
	if now.After(t.createTime.Add(time.Minute * 5)) {
		return true
	}
	return false
}

func (t *tenantServiceResource) DeleteResources() error {
	if err := t.manager.kubeclient.Namespaces().Delete(t.namespaces, &meta_v1.DeleteOptions{}); err != nil {
		logrus.Error(err)
		return err
	} else {
		logrus.Info("delete namespaces success：", t.namespaces)
		return nil
	}
	return nil
}

func (t *tenantServiceResource) IsClean() bool {
	isNotExist := db.GetManager().TenantDao().GetTenantByUUIDIsExist(t.namespaces)
	if isNotExist {
		return true
	}
	return false
}

func (t *tenantServiceResource) Name() string {
	return t.id
}

func (t *tenantServiceResource) Type() string {
	return "namespaces"
}

func queryTenantServiceResource(m *Manager) []Resource {
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
		s := &tenantServiceResource{
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
	if err := r.manager.kubeclient.ReplicationControllers(r.namespaces).Delete(r.id, &meta_v1.DeleteOptions{}); err != nil {
		logrus.Error(err)
		return err
	} else {
		logrus.Info("delete replicationcontroller success：", r.id)
		return nil
	}
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

func queryRcResource(m *Manager) []Resource {
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
			if InSlice(v.Name, val) {
				s := &rcResource{
					manager:    m,
					namespaces: v.Namespace,
					id:         v.Name,
					createTime: time.Now(),
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
	cancel        context.CancelFunc
	l             list.List
}

func NewManager(ctx context.Context, kubeclient *kubernetes.Clientset) *Manager {
	m := &Manager{
		ctx:        ctx,
		kubeclient: kubeclient,
	}
	queryResource := []func(*Manager) []Resource{
		queryRcResource,
		queryTenantServiceResource,
		queryStatefulResource,
		queryDeploymentResource,
		queryK8sServiceResource,
	}
	m.queryResource = queryResource
	return m
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
	}, time.Hour*24)

}

func (m *Manager) PerformTasks() {

	util.Exec(m.ctx, func() error {
		for m.l.Len() > 1 {
			rs := m.l.Back()
			if res, ok := rs.Value.(Resource); ok {
				if res.IsTimeout() {
					if res.IsClean() {
						if err := res.DeleteResources(); err != nil {
							logrus.Error(err)
							m.l.Remove(rs)
						} else {
							m.l.Remove(rs)
						}
					} else {
						m.l.Remove(rs)
					}
				}
			} else {
				logrus.Error("Type conversion failed")
			}
		}
		return nil
	}, time.Hour*12)
}

func (m *Manager) Start() error {
	logrus.Info("clean up module starts....")
	go m.CollectingTasks()
	go m.PerformTasks()
	return nil

}
func (m *Manager) Stop() error {
	logrus.Info("CleanResource is stoping.")
	m.cancel()
	return nil
}
