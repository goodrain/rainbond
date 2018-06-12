package clean

import (
	"k8s.io/client-go/kubernetes"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"fmt"
	"github.com/Sirupsen/logrus"
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
	Namespaces1, err := c.kubeclient.CoreV1().Namespaces().List(meta_v1.ListOptions{})
	if err != nil{
		fmt.Println(err)
	}

	fmt.Println("namespaces：",Namespaces1.Items)

	fmt.Println(Namespaces1.Items[1].Name)
	fmt.Println(Namespaces1.Items[1].Spec)
	fmt.Println(Namespaces1.Items[1].GenerateName)
	fmt.Println(Namespaces1.Items[1].Labels)
	fmt.Println(Namespaces1.Items[1].Namespace)
	fmt.Println(Namespaces1.Items[1].UID)
	


}
