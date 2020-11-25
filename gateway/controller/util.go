package controller

import (
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClientSet returns a new kubernetes clientSet
func NewClientSet(kubeconfig string) (*kubernetes.Clientset, error) {
	conf, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	conf.QPS = 50
	conf.Burst = 100
	clientSet, err := kubernetes.NewForConfig(conf)
	if err != nil {
		return nil, err
	}

	//_, err = clientSet.CoreV1().Namespaces().List(v1.ListOptions{})
	//if err != nil {
	//	return nil, err
	//}

	logrus.Debug("Kube client api create success.")

	return clientSet, nil
}
