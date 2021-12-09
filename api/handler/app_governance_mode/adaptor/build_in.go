package adaptor

type buildInServiceMeshMode struct{}

// NewBuildInServiceMeshMode -
func NewBuildInServiceMeshMode() AppGoveranceModeHandler {
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
