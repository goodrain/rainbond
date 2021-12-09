package kubernetesnative

import appgovernancemode "github.com/goodrain/rainbond/api/handler/app_governance_mode"

type kubernetesNativeMode struct {
}

// New Kubernetes Native Mode Handler
func New() appgovernancemode.AppGoveranceModeHandler {
	return &kubernetesNativeMode{}
}

// IsInstalledControlPlane -
func (k *kubernetesNativeMode) IsInstalledControlPlane() bool {
	return true
}

// GetInjectLabels-
func (k *kubernetesNativeMode) GetInjectLabels() map[string]string {
	return nil
}
