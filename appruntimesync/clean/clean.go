package clean

import (
	"k8s.io/client-go/kubernetes"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/db"
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
	logrus.Info("clean 开始工作...")
	go c.Run()
	go c.cleanStatefulset()
	go c.cleanService()
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
func SliceIntersect(slice1, slice2 []string) (diffslice []string) {
	for _, v := range slice1 {
		if InSlice(v, slice2) {
			diffslice = append(diffslice, v)
		}
	}
	return
}



func (c *CheanManager) Run() {
	nameList := make([]string, 0, 200)
	allList := make([]string, 0, 300)
	Namespaces1, err := c.kubeclient.CoreV1().Namespaces().List(meta_v1.ListOptions{})
	if err != nil {
		fmt.Println(err)
	}

	for _, v := range Namespaces1.Items {
		if len(v.Name) != 32{
			continue
		}
		nameList = append(nameList, v.Name)
	}
	fmt.Println(len(nameList), nameList[0], nameList[2])

	AllTenantsList, err := db.GetManager().TenantDao().GetALLTenants()

	for _, v := range AllTenantsList {
		allList = append(allList, v.UUID)
	}

	diffList := SliceDiff(nameList,allList)
	fmt.Println(diffList)

	for _,v := range diffList {
		err := c.kubeclient.Namespaces().Delete(v,&meta_v1.DeleteOptions{})
		if err != nil{
			fmt.Println("删除错误",err)
		}
		break
	}



}

func (c *CheanManager) cleanStatefulset(){

	StatefulSetsMap := make(map[string][]string)
	ReplicationControllersMap := make(map[string][]string)
	StadeleteList := make(map[string]string)
	RepdeleteList := make(map[string]string)

	isDeleteList,err := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationByIsDelete(true)
	if err!= nil{
		logrus.Error(err)
	}



	for _,v := range isDeleteList {
		if v.ReplicationType == "statefulset"{

			if _,ok := StatefulSetsMap[v.TenantID];ok{
				StatefulSetsMap[v.TenantID] = append(StatefulSetsMap[v.TenantID], v.ReplicationID)
			}else {
				StatefulSetsMap[v.TenantID] = []string{v.ReplicationID}
			}

		}

		if v.ReplicationType == "replicationcontroller"{
			if _,ok := ReplicationControllersMap[v.TenantID];ok{
				ReplicationControllersMap[v.TenantID] = append(ReplicationControllersMap[v.TenantID], v.ReplicationID)
			}else {
				ReplicationControllersMap[v.TenantID] = []string{v.ReplicationID}
			}
		}
	}
	i:=1
	for k,valuse := range StatefulSetsMap{
		StatefulSetsList,err := c.kubeclient.StatefulSets(k).List(meta_v1.ListOptions{})
		if err != nil{
			logrus.Error("错误3",err)
		}
		for _,v := range StatefulSetsList.Items{
			fmt.Println("sta:",v.Name,i)
			i++
			if InSlice(v.Name,valuse){
				StadeleteList[k] = v.Name
			}
		}
	}

	for k,valuse := range ReplicationControllersMap{
		ReplicationControllersList,err := c.kubeclient.ReplicationControllers(k).List(meta_v1.ListOptions{})
		if err != nil{
			logrus.Error("错误4",err)
		}
		for _,v := range ReplicationControllersList.Items{
			fmt.Println("rep:",v.Name,i)
			i++
			if InSlice(v.Name,valuse){
				RepdeleteList[k] = v.Name
			}
		}
	}

	fmt.Println("StadeleteList",StadeleteList)
	fmt.Println("RepdeleteList",RepdeleteList)

	fmt.Println("xxx",StatefulSetsMap)
	fmt.Println("vvvv",ReplicationControllersMap)



	service,err := c.kubeclient.CoreV1().Services("3e2fe69f5d3b4bf7b6bf7b5ba97e8b74").List(meta_v1.ListOptions{})
	if err != nil{
		logrus.Error("错误5",err)
	}
	for _,v :=range service.Items{
		fmt.Println(v.Name)
		fmt.Println(v.Labels)
		fmt.Println(v.Namespace)
		fmt.Println(v.UID)
	}


}


func (c *CheanManager) cleanService() {


}