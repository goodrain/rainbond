package adaptor

type kubernetesNativeMode struct {
}

// NewKubernetesNativeMode -
func NewKubernetesNativeMode() AppGoveranceModeHandler {
	return &kubernetesNativeMode{}
}

// IsInstalledControlPlane -
func (k *kubernetesNativeMode) IsInstalledControlPlane() bool {
	return true
}

// GetInjectLabels -
func (k *kubernetesNativeMode) GetInjectLabels() map[string]string {
	return nil
}
