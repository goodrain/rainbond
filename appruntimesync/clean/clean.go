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

type CheanUp interface {
	IsTimeout() bool
	DeleteResources(map[string]string)

}

type CheanManager struct {
	ctx        context.Context
	kubeclient *kubernetes.Clientset
	data       map[string]string
	period     time.Time
	genre      string
}


var TaskSlice = make([]*CheanManager, 0, 100)

func NewCheanManager(ctx context.Context, kubeclient *kubernetes.Clientset) *CheanManager {
	m := &CheanManager{
		ctx:        ctx,
		kubeclient: kubeclient,
	}

	return m
}

func (c *CheanManager) Start() {
	logrus.Info("clean up module starts....")
	c.CollectingTasks()
	fmt.Println("TaskSlice",TaskSlice)
	c.PerformTasks()
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

// SliceIntersect returns slice that are present in all the slice1 and slice2.
func SliceIntersect(slice1, slice2 []string) (IntersectSlice []string) {
	for _, v := range slice1 {
		if InSlice(v, slice2) {
			IntersectSlice = append(IntersectSlice, v)
		}
	}
	return
}

func MapIntersect(map1, map2 map[string]string) (IntersectMap map[string]string) {
	intersectMap := make(map[string]string)
	for k, v := range map2 {
		v2, ok := map1[k]
		if ok {
			if v == v2 {
				intersectMap[k] = v
			}
		}

	}
	return intersectMap
}

func (c *CheanManager) cleanNamespaces() {
	nameList := make([]string, 0, 200)
	allList := make([]string, 0, 300)
	diffMap := make(map[string]string)
	Namespaces, err := c.kubeclient.CoreV1().Namespaces().List(meta_v1.ListOptions{})
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
		diffMap[v] = v
	}
	fmt.Println("diffMap:",diffMap)

	TaskSlice = append(TaskSlice, &CheanManager{
		data:   diffMap,
		period: time.Now(),
		genre:  "namespaces",
	})
	fmt.Println("1结束")

}

func (c *CheanManager) cleanStaAndRep() {

	StatefulSetsMap := make(map[string][]string)
	ReplicationControllersMap := make(map[string][]string)
	StadeleteMap := make(map[string]string)
	RepdeleteMap := make(map[string]string)

	isDeleteList, err := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationByIsDelete(true)
	if err != nil {
		logrus.Error(err)
	}

	for _, v := range isDeleteList {
		if v.ReplicationType == "statefulset" {

			if _, ok := StatefulSetsMap[v.TenantID]; ok {
				StatefulSetsMap[v.TenantID] = append(StatefulSetsMap[v.TenantID], v.ReplicationID)
			} else {
				StatefulSetsMap[v.TenantID] = []string{v.ReplicationID}
			}

		}

		if v.ReplicationType == "replicationcontroller" {
			if _, ok := ReplicationControllersMap[v.TenantID]; ok {
				ReplicationControllersMap[v.TenantID] = append(ReplicationControllersMap[v.TenantID], v.ReplicationID)
			} else {
				ReplicationControllersMap[v.TenantID] = []string{v.ReplicationID}
			}
		}
	}
	for k, valuse := range StatefulSetsMap {
		StatefulSetsList, err := c.kubeclient.StatefulSets(k).List(meta_v1.ListOptions{})
		if err != nil {
			logrus.Error(err)
		}
		for _, v := range StatefulSetsList.Items {
			if InSlice(v.Name, valuse) {
				StadeleteMap[k] = v.Name
			}
		}
	}

	for k, valuse := range ReplicationControllersMap {
		ReplicationControllersList, err := c.kubeclient.ReplicationControllers(k).List(meta_v1.ListOptions{})
		if err != nil {
			logrus.Error(err)
		}
		for _, v := range ReplicationControllersList.Items {
			if InSlice(v.Name, valuse) {
				RepdeleteMap[k] = v.Name
			}
		}
	}
	fmt.Println("statefulset：",StadeleteMap)
	TaskSlice = append(TaskSlice, &CheanManager{
		data:   StadeleteMap,
		period: time.Now(),
		genre:  "statefulset",
	})
	fmt.Println("2结束")
	fmt.Println("replicationcontroller：",RepdeleteMap)
	TaskSlice = append(TaskSlice, &CheanManager{
		data:   RepdeleteMap,
		period: time.Now(),
		genre:  "replicationcontroller",
	})
	fmt.Println("3结束")

}

func (c *CheanManager) cleanService() {

	ServivesMap := make(map[string][]string)
	ServivesDeleteMap := make(map[string]string)

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
		ServicesList, err := c.kubeclient.Services(k).List(meta_v1.ListOptions{})
		if err != nil {
			logrus.Error(err)
		}
		for _, v := range ServicesList.Items {
			if !InSlice(v.Name, valuse) {
				ServivesDeleteMap[k] = v.Name
			}
		}
	}

	fmt.Println("services：",ServivesDeleteMap)
	TaskSlice = append(TaskSlice, &CheanManager{
		data:   ServivesDeleteMap,
		period: time.Now(),
		genre:  "services",
	})
	fmt.Println("4结束")

}

func (c *CheanManager) IsTimeout() bool {
	now := time.Now()
	if now.After(c.period.Add(time.Second *0)) {
		return true
	}
	return false
}

func (c *CheanManager) DeleteResources(deleteMap map[string]string) {

	if c.genre == "namespaces" {
		for _, v := range deleteMap {
			isExist := db.GetManager().TenantDao().GetTenantByUUIDIsExist(v)
			fmt.Println("isExist",isExist)
			if isExist {
				if err := c.kubeclient.Namespaces().Delete(v, &meta_v1.DeleteOptions{}); err != nil {
					logrus.Error(err)
				} else {
					logrus.Info("delete namespaces success：", v)
				}

			}

		}
	}

	if c.genre == "statefulset" {
		for k, v := range deleteMap {
			isExist := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationIsExist(k, "statefulset", v, false)
			if isExist {
				if err := c.kubeclient.StatefulSets(k).Delete(v, &meta_v1.DeleteOptions{}); err != nil {
					logrus.Error(err)
				} else {
					logrus.Info("delete statefulset success：", v)
				}
			}
		}
	}

	if c.genre == "replicationcontroller" {
		for k, v := range deleteMap {
			isExist := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationIsExist(k, "replicationcontroller", v, false)
			if isExist {
				if err := c.kubeclient.ReplicationControllers(k).Delete(v, &meta_v1.DeleteOptions{}); err != nil {
					logrus.Error(err)
				} else {
					logrus.Info("delete replicationcontroller success：", v)
				}
			}
		}
	}

	if c.genre == "services" {
		for k, v := range deleteMap {
			isExist := db.GetManager().K8sServiceDao().K8sServiceIsExist(k, v)
			if isExist {
				if err := c.kubeclient.Services(k).Delete(v, &meta_v1.DeleteOptions{}); err != nil {
					logrus.Error(err)
				} else {
					logrus.Info("delete service success：", v)
				}
			}
		}
	}
}

func (c *CheanManager) CollectingTasks() {
	run := func() { util.Exec(c.ctx, func() error {
		c.cleanNamespaces()
		c.cleanStaAndRep()
		c.cleanService()
		return nil
	}, time.Minute*24)}
	go run()
}

func (c *CheanManager) PerformTasks() {
	run := func() {util.Exec(c.ctx, func() error {
		fmt.Println("长度：",len(TaskSlice))
		for _, v := range TaskSlice {
			if v.IsTimeout() {
				v.DeleteResources(v.data)
			}
		}
		return nil
	}, time.Minute*12)}
	go run()
}
