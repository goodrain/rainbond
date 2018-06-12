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


func checkSliceBInA(a []string, b []string) (isIn bool, diffSlice []string) {

	lengthA := len(a)

	for _, valueB := range b {

		temp := valueB //遍历取出B中的元素

		for j := 0; j < lengthA; j++ {
			if temp == a[j] { //如果相同 比较下一个
				break
			} else {
				if lengthA == (j + 1) { //如果不同 查看a的元素个数及当前比较元素的位置 将不同的元素添加到返回slice中
					diffSlice = append(diffSlice, temp)
					fmt.Println("---->", diffSlice)
				}
			}
		}
	}

	if len(diffSlice) == 0 {
		isIn = true
	} else {
		isIn = false
	}

	return isIn, diffSlice
}

func (c *CheanManager) Run() {
	nameList := make([]string, 0, 200)
	alllist := make([]string,0,300)
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

	isIn,diffSlice := checkSliceBInA(nameList,alllist)
	fmt.Println(isIn,diffSlice)

}
