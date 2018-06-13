package clean

import (
	"k8s.io/client-go/kubernetes"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/db"
	"time"
	"github.com/goodrain/rainbond/util"
	"context"
	"fmt"
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
	if now.After(k.createTime.Add(time.Second * 0)) {
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
	for k, valuse := range ServivesMap {
		ServicesList, err := m.kubeclient.Services(k).List(meta_v1.ListOptions{})
		if err != nil {
			logrus.Error(err)
		}
		for _, v := range ServicesList.Items {
			if !InSlice(v.Name, valuse) {
				s := &k8sServiceResource{
					createTime: time.Now(),
					namespaces: k,
					id:         v.Name,
				}
				m.waiting = append(m.waiting, s)
			}
		}
	}
	return nil
}

func (d *deploymentResource) IsTimeout() bool {
	now := time.Now()
	if now.After(d.createTime.Add(time.Second * 0)) {
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
	for k, valuse := range DeploymentMap {
		DeploymentList, err := m.kubeclient.AppsV1beta1().Deployments(k).List(meta_v1.ListOptions{})
		if err != nil {
			logrus.Error(err)
		}
		for _, v := range DeploymentList.Items {
			if InSlice(v.Name, valuse) {
				s := &deploymentResource{
					createTime: time.Now(),
					namespaces: k,
					id:         v.Name,
				}
				m.waiting = append(m.waiting, s)
			}
		}
	}
	return nil
}

func (s *statefulResource) IsTimeout() bool {
	now := time.Now()
	if now.After(s.createTime.Add(time.Second * 0)) {
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
	for k, valuse := range StatefulSetsMap {
		StatefulSetsList, err := m.kubeclient.StatefulSets(k).List(meta_v1.ListOptions{})
		if err != nil {
			logrus.Error(err)
		}
		for _, v := range StatefulSetsList.Items {
			if InSlice(v.Name, valuse) {
				s := &statefulResource{
					createTime: time.Now(),
					namespaces: k,
					id:         v.Name,
				}
				m.waiting = append(m.waiting, s)
			}
		}
	}
	return nil
}

func (t *tenantServiceResource) IsTimeout() bool {
	now := time.Now()
	if now.After(t.createTime.Add(time.Second * 0)) {
		return true
	}
	return false
}

func (t *tenantServiceResource) DeleteResources() error {
	//if err := t.manager.kubeclient.Namespaces().Delete(t.namespaces, &meta_v1.DeleteOptions{}); err != nil {
	//	logrus.Error(err)
	//	return err
	//} else {
	//	logrus.Info("delete namespaces success：", t.namespaces)
	//	return nil
	//}
	fmt.Println("删除", t.id, t.namespaces)
	return nil
}

func (t *tenantServiceResource) IsClean() bool {
	isNotExist := db.GetManager().TenantDao().GetTenantByUUIDIsExist(t.namespaces)
	fmt.Println("isNotExist", isNotExist)
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
			createTime: time.Now(),
			id:         v,
			namespaces: v,
		}
		fmt.Println("sss",s)
		m.waiting = append(m.waiting, s)
		fmt.Println("列表",m.waiting)
	}

	return nil
}

func (r *rcResource) IsTimeout() bool {
	now := time.Now()
	if now.After(r.createTime.Add(time.Second * 0)) {
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

	for k, valuse := range ReplicationControllersMap {
		ReplicationControllersList, err := m.kubeclient.ReplicationControllers(k).List(meta_v1.ListOptions{})
		if err != nil {
			logrus.Error(err)
		}
		for _, v := range ReplicationControllersList.Items {
			if InSlice(v.Name, valuse) {
				s := &rcResource{
					namespaces: k,
					id:         v.Name,
					createTime: time.Now(),
				}
				m.waiting = append(m.waiting, s)
			}
		}
	}
	return nil
}

type Manager struct {
	ctx           context.Context
	kubeclient    *kubernetes.Clientset
	waiting       []Resource
	queryResource []func(*Manager) []Resource
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

func (m *Manager) Start() {
	logrus.Info("clean up module starts....")
	go m.CollectingTasks()
	go m.PerformTasks()

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
			for _, v := range m.queryResource {
				v(m)
				fmt.Println("xx",v(m))
			}
			return nil
		}, time.Second*24)

}

func (m *Manager) PerformTasks() {

		util.Exec(m.ctx, func() error {
			fmt.Println("长度", m.waiting)
			for _, v := range m.waiting {
				if v.IsTimeout() {
					if v.IsClean() {
						v.DeleteResources()
					}
				}
			}
			fmt.Println("结束")
			m.waiting = nil
			return nil
		}, time.Second*12)
}
