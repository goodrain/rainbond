package handler

import (
	"context"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"testing"
)

func TestClusterAction_HandleResourceYaml(t *testing.T) {
	yamlFile := `
apiVersion: rainbond.io/v1alpha1
kind: RBDPlugin
metadata:
  name: rbdplugin-yk8888888
spec:
  # TODO(user): Add fields here
  author: yangk88888
  description: "This is yangkaa app 888888"
`
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/yangk/.kube/config")
	if err != nil {
		return
	}
	clientset := kubernetes.NewForConfigOrDie(config)
	// rest mapper
	gr, err := restmapper.GetAPIGroupResources(clientset)
	if err != nil {
		return
	}
	mapper := restmapper.NewDiscoveryRESTMapper(gr)
	handler := NewClusterHandler(clientset, "k8s.io", "conf.GrctlImage", config, mapper, nil)
	handler.AddAppK8SResource(context.Background(), "default", "sssssss", yamlFile)
}
