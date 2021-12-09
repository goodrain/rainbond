package appgovernancemode

import (
	"github.com/goodrain/rainbond/api/handler/app_governance_mode/build_in"
	"github.com/goodrain/rainbond/api/handler/app_governance_mode/istio"
	kubernetesnative "github.com/goodrain/rainbond/api/handler/app_governance_mode/kubernetes_native"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db/model"
	clientset "k8s.io/client-go/kubernetes"
)

// AppGoveranceModeHandler Application governance mode processing interface
type AppGoveranceModeHandler interface {
	IsInstalledControlPlane() bool
	GetInjectLabels() map[string]string
}

// NewAppGoveranceModeHandler -
func NewAppGoveranceModeHandler(governanceMode string, kubeClient clientset.Interface) (AppGoveranceModeHandler, error) {
	switch governanceMode {
	case model.GovernanceModeIstioServiceMesh:
		return istio.New(kubeClient), nil
	case model.GovernanceModeBuildInServiceMesh:
		return buildin.New(), nil
	case model.GovernanceModeKubernetesNativeService:
		return kubernetesnative.New(), nil
	default:
		return nil, bcode.ErrInvalidGovernanceMode
	}
}

// IsGovernanceModeValid checks if the governanceMode is valid.
func IsGovernanceModeValid(governanceMode string) bool {
	switch governanceMode {
	case model.GovernanceModeBuildInServiceMesh:
		return true
	case model.GovernanceModeKubernetesNativeService:
		return true
	case model.GovernanceModeIstioServiceMesh:
		return true
	}
	return false
}
