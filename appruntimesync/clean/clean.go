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

		nameList = append(nameList, v.Name)
	}
	fmt.Println(len(nameList), nameList[0], nameList[2])

	AllTenantsList, err := db.GetManager().TenantDao().GetALLTenants()

	for _, v := range AllTenantsList {
		allList = append(allList, v.UUID)
	}

	diffList := SliceDiff(nameList,allList)
	fmt.Println(diffList)

	StatefulSets, err := c.kubeclient.StatefulSets("824b2e9dcc4d461a852ddea20369d377").List(meta_v1.ListOptions{})
	ReplicationControllers, err := c.kubeclient.ReplicationControllers("c69c40ecedae41ca9fbb6c3cec0926f2").List(meta_v1.ListOptions{})

	for _,v:=range StatefulSets.Items{
		fmt.Println(v.Name)
		fmt.Println(v.Labels)
	}
	err2 := c.kubeclient.StatefulSets("824b2e9dcc4d461a852ddea20369d377").Delete("grd1b4e0",meta_v1.NewDeleteOptions(0))

	fmt.Println("----------------------")
	for _,v:=range ReplicationControllers.Items{
		fmt.Println(v.Name)
		fmt.Println(v.Labels)
	}
	if err2!=nil{
		fmt.Println("错误",err2)
	}


}
