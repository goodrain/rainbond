package adaptor

import (
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
		return NewIstioGoveranceMode(kubeClient), nil
	case model.GovernanceModeBuildInServiceMesh:
		return NewBuildInServiceMeshMode(), nil
	case model.GovernanceModeKubernetesNativeService:
		return NewKubernetesNativeMode(), nil
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
