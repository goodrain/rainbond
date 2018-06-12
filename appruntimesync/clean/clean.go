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


func (c *CheanManager) Run() {
	nameList := make([]string, 0, 200)
	alllist := make([]string, 0, 300)
	Namespaces1, err := c.kubeclient.CoreV1().Namespaces().List(meta_v1.ListOptions{})
	if err != nil {
		fmt.Println(err)
	}

	for _, v := range Namespaces1.Items {

		nameList = append(nameList, v.Name)
	}
	fmt.Println(len(nameList), nameList[0], nameList[2])

	ALLTeantsList, err := db.GetManager().TenantDao().GetALLTenants()

	for _, v := range ALLTeantsList {
		alllist = append(alllist, v.UUID)
	}
	

	StatefulSets, err := c.kubeclient.StatefulSets("824b2e9dcc4d461a852ddea20369d377").List(meta_v1.ListOptions{})
	ReplicationControllers, err := c.kubeclient.ReplicationControllers("c69c40ecedae41ca9fbb6c3cec0926f2").List(meta_v1.ListOptions{})

	fmt.Println(StatefulSets.Items[0].Name)
	fmt.Println(StatefulSets.Items[0].GenerateName)
	fmt.Println(StatefulSets.Items[0].UID)
	fmt.Println(StatefulSets.Items[0].Namespace)
	fmt.Println(StatefulSets.Items[0].Labels)
	fmt.Println("----------------------")
	fmt.Println(ReplicationControllers.Items[0].Name)
	fmt.Println(ReplicationControllers.Items[0].GenerateName)
	fmt.Println(ReplicationControllers.Items[0].UID)
	fmt.Println(ReplicationControllers.Items[0].Namespace)
	fmt.Println(ReplicationControllers.Items[0].Labels)


}
