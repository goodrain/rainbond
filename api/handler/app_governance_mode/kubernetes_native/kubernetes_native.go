package kubernetes_native

type KubernetesNativeMode struct {
}

func (k *KubernetesNativeMode) IsInstalledControlPlane() bool {
	return true
}

func (k *KubernetesNativeMode) GetInjectLabels() map[string]string {
	return nil
}
