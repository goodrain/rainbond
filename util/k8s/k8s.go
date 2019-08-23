package k8s

import (
	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClientset -
func NewClientset(kubecfg string) (kubernetes.Interface, error) {
	c, err := clientcmd.BuildConfigFromFlags("", kubecfg)
	if err != nil {
		logrus.Errorf("error reading kube config file: %s", err.Error())
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		logrus.Error("error creating kube api client", err.Error())
		return nil, err
	}
	return clientset, nil
}
