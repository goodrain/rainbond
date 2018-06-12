package clean

import (
	"k8s.io/client-go/kubernetes"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/db"
	"time"
	"fmt"
)

type CheanManager struct {
	kubeclient *kubernetes.Clientset
}

func NewCheanManager(kubeclient *kubernetes.Clientset) *CheanManager {
	m := &CheanManager{
		kubeclient: kubeclient,
	}
	return m
}

func (c *CheanManager) Start() {
	logrus.Info("clean up module starts....")
	go c.DeleteNamespaces()
	//go c.cleanStatefulset()
	//go c.cleanService()
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
func SliceDiff(slice1, slice2 []string) (diffslice []string) {
	for _, v := range slice1 {
		if !InSlice(v, slice2) {
			diffslice = append(diffslice, v)
		}
	}
	return
}

// SliceIntersect returns slice that are present in all the slice1 and slice2.
func SliceIntersect(slice1, slice2 *[]string) (Intersectslice []string) {
	for _, v := range *slice1 {
		if InSlice(v, *slice2) {
			Intersectslice = append(Intersectslice, v)
		}
	}
	return
}

func (c *CheanManager) cleanNamespaces() (*[]string) {
	nameList := make([]string, 0, 200)
	allList := make([]string, 0, 300)
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
	return &diffList

	//for _, v := range diffList {
	//	err := c.kubeclient.Namespaces().Delete(v, &meta_v1.DeleteOptions{})
	//	if err != nil {
	//		fmt.Println(err)
	//	}
	//
	//	logrus.Info("delete namespaces success:", v)
	//}

}

func (c *CheanManager) cleanStatefulset() (*map[string]string,*map[string]string) {

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

	return &StadeleteMap, &RepdeleteMap

	//for k, v := range StadeleteMap {
	//	if err := c.kubeclient.StatefulSets(k).Delete(v, &meta_v1.DeleteOptions{}); err != nil {
	//		logrus.Error(err)
	//	}
	//
	//}
	//
	//for k, v := range RepdeleteMap {
	//	if err := c.kubeclient.ReplicationControllers(k).Delete(v, &meta_v1.DeleteOptions{}); err != nil {
	//		logrus.Error(err)
	//	}
	//}

}

func (c *CheanManager) cleanService() *map[string]string {

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

	return &ServivesDeleteMap
	//
	//for k, v := range ServivesDeleteMap {
	//	err := c.kubeclient.Services(k).Delete(v, &meta_v1.DeleteOptions{})
	//	if err != nil {
	//		logrus.Error(err)
	//	}
	//	logrus.Info("delete service success：", v)
	//}

}


func (c *CheanManager) DeleteNamespaces(){

	diffList :=c.cleanNamespaces()
	fmt.Println(diffList)
	time.AfterFunc(time.Second *10, func() {
		newdiffList :=c.cleanNamespaces()
		deleteList := SliceIntersect(newdiffList,diffList)
		fmt.Println("delete:",deleteList)
	})

}
