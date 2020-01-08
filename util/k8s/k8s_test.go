package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"testing"
)

func TestNewRestConfig(t *testing.T) {
	os.Setenv("KUBERNETES_SERVICE_HOST", "192.168.2.200")
	restConfig, err := NewRestConfig("")
	if err != nil {
		t.Fatal("create restconfig error: ", err.Error())
	}
	clientset, err := NewClientsetWithRestConfig(restConfig)
	if err != nil {
		t.Fatal("create clientset error: ", err.Error())
	}
	pod, err := clientset.CoreV1().Pods("5d7bd886e6dc4425bb6c2ac5fc9fa593").Get("gr2e4b9f-0", metav1.GetOptions{})
	if err != nil {
		t.Fatal("get pod info error: ", err.Error())
	}
	t.Logf("pod info : %+v", pod)
}
