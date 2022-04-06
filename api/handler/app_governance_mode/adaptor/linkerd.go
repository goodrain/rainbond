package adaptor

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"os"
	"time"
)

type linkerdServiceMeshMode struct {
	kubeClient clientset.Interface
}

// NewLinkerdGoveranceMode -
func NewLinkerdGoveranceMode(kubeClient clientset.Interface) AppGoveranceModeHandler {
	return &linkerdServiceMeshMode{
		kubeClient: kubeClient,
	}
}

// IsInstalledControlPlane -
func (i *linkerdServiceMeshMode) IsInstalledControlPlane() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mwcName := os.Getenv("LINKERD_MWC")
	if mwcName == "" {
		mwcName = "linkerd-proxy-injector-webhook-config"
	}
	_, err := i.kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, mwcName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}

// GetInjectLabels -
func (i *linkerdServiceMeshMode) GetInjectLabels() map[string]string {
	return nil
}
