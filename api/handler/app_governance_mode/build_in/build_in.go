package build_in

type BuildInServiceMeshMode struct {
}

func (b *BuildInServiceMeshMode) IsInstalledControlPlane() bool {
	return true
}

func (b *BuildInServiceMeshMode) GetInjectLabels() map[string]string {
	return nil
}
