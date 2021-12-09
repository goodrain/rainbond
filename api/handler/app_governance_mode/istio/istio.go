package istio

import (
	"context"
	appgovernancemode "github.com/goodrain/rainbond/api/handler/app_governance_mode"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"os"
	"time"
)

type istioServiceMeshMode struct {
	kubeClient clientset.Interface
}

// New Istio Goverance Mode Handler
func New(kubeClient clientset.Interface) appgovernancemode.AppGoveranceModeHandler{
	return &istioServiceMeshMode{
		kubeClient: kubeClient,
	}
}

// IsInstalledControlPlane -
func (i *istioServiceMeshMode) IsInstalledControlPlane() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmName := os.Getenv("ISTIO_CM")
	if cmName == "" {
		cmName = "istio-ca-root-cert"
	}
	_, err := i.kubeClient.CoreV1().ConfigMaps("default").Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}

// GetInjectLabels -
func (i *istioServiceMeshMode) GetInjectLabels() map[string]string {
	return map[string]string{"sidecar.istio.io/inject": "true"}
}
