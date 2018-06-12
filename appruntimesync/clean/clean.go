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
	nameList := make([]string,0,200)
	Namespaces1, err := c.kubeclient.CoreV1().Namespaces().List(meta_v1.ListOptions{})
	if err != nil{
		fmt.Println(err)
	}

for _,v := range Namespaces1.Items{

	nameList = append(nameList, v.Name)
}
fmt.Println(len(nameList),nameList[0],nameList[2])

deleteList,err := db.GetManager().TenantDao().GetTenant(nameList)
fmt.Println(deleteList)

//c.kubeclient.CoreV1().Namespaces().Delete()
}
