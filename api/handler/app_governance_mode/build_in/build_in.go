package buildin

import appgovernancemode "github.com/goodrain/rainbond/api/handler/app_governance_mode"

type buildInServiceMeshMode struct{}

// New Build In ServiceMeshMode Handler
func New() appgovernancemode.AppGoveranceModeHandler {
	return &buildInServiceMeshMode{}
}

// IsInstalledControlPlane -
func (b *buildInServiceMeshMode) IsInstalledControlPlane() bool {
	return true
}

// GetInjectLabels -
func (b *buildInServiceMeshMode) GetInjectLabels() map[string]string {
	return nil
}
