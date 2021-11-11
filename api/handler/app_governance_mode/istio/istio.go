package istio

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"os"
	"time"
)

type IstioServiceMeshMode struct {
	KubeClient clientset.Interface
}

func (i *IstioServiceMeshMode) IsInstalledControlPlane() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmName := os.Getenv("ISTIO_CM")
	if cmName == "" {
		cmName = "istio-ca-root-cert"
	}
	_, err := i.KubeClient.CoreV1().ConfigMaps("default").Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}

func (i *IstioServiceMeshMode) GetInjectLabels() map[string]string {
	return map[string]string{"sidecar.istio.io/inject": "true"}
}
