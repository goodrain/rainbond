package clients

import (
	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"rainbond/cmd/grctl/option"
	"k8s.io/client-go/rest"
)

var client *kubernetes.Clientset

func InitClient(kube option.Kubernets) error {
	var config rest.Config
	config.Host = kube.Master
	// creates the clientset
	var err error
	client, err = kubernetes.NewForConfig(&config)
	if err != nil {
		logrus.Error("Create kubernetes client error.", err.Error())
		return err
	}
	return nil
}