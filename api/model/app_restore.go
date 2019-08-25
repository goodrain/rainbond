package model

// RestoreEnvsReq defines a struct to receive the request body
// to restore enviroment variables
type RestoreEnvsReq struct {
	Scope string        `validate:"scope|required|in:outer,inner,both,build"`
	Envs  []*RestoreEnv `validate:"envs|required" json:"envs"`
}

// RestoreEnv holds infomations of every enviroment variables.
type RestoreEnv struct {
	ContainerPort int    `validate:"container_port|numeric_between:1,65535" json:"container_port"`
	Name          string `validate:"name" json:"name"`
	AttrName      string `validate:"env_name|required" json:"env_name"`
	AttrValue     string `validate:"env_value|required" json:"env_value"`
	IsChange      bool   `validate:"is_change|bool" json:"is_change"`
	Scope         string `validate:"scope|in:outer,inner,both,build" json:"scope"`
}

// RestorePortsReq defines a struct to receive the request body
// to restore service ports
type RestorePortsReq struct {
	Ports []*RestorePort `validate:"ports|required" json:"ports"`
}

// RestorePort holds information of port.
type RestorePort struct {
	ContainerPort  int    `gorm:"column:container_port" validate:"container_port|required|numeric_between:1,65535" json:"container_port"`
	MappingPort    int    `gorm:"column:mapping_port" validate:"mapping_port|required|numeric_between:1,65535" json:"mapping_port"`
	Protocol       string `gorm:"column:protocol" validate:"protocol|required|in:http,https,stream,grpc" json:"protocol"`
	PortAlias      string `gorm:"column:port_alias" validate:"port_alias|required|alpha_dash" json:"port_alias"`
	IsInnerService bool   `gorm:"column:is_inner_service" validate:"is_inner_service|bool" json:"is_inner_service"`
	IsOuterService bool   `gorm:"column:is_outer_service" validate:"is_outer_service|bool" json:"is_outer_service"`
}

// RestoreVolumesReq defines a struct to receive the request body
// to restore service volumes
type RestoreVolumesReq struct {
	Volumes []*RestoreVolume `validate:"volumes|required" json:"volumes"`
}

// RestoreVolume holds infomations of port.
type RestoreVolume struct {
	Category    string `json:"category"`
	VolumeName  string `json:"volume_name" validate:"volume_name|required|max:50"`
	VolumePath  string `json:"volume_path" validate:"volume_path|required|regex:^/"`
	VolumeType  string `json:"volume_type" validate:"volume_type|required|in:share-file,config-file"`
	FileContent string `json:"file_content"`
	HostPath    string `json:"host_path"`
	IsReadOnly  bool   `json:"is_read_only"`
}

// RestoreDepsReq defines a struct to receive the request body
// to restore service dependencies.
type RestoreDepsReq struct {
	Deps []*RestoreDep `validate:"deps|required" json:"deps"`
}

// RestoreDep holds infomations of service dependency.
type RestoreDep struct {
	DepServiceID   string `validata:"dep_service_id|required" json:"dep_service_id"`
	DepServiceType string `validata:"dep_service_type|required" json:"dep_service_type"`
}

// RestoreDepVolsReq defines a struct to receive the request body
// to restore service dependent volumes.
type RestoreDepVolsReq struct {
	DepVols []*RestoreDepVol `validate:"dep_vols|required" json:"dep_vols"`
}

// RestoreDepVol holds information of service dependent volume.
type RestoreDepVol struct {
	DepServiceID string `json:"dep_service_id"  validate:"dep_service_id|required"`
	VolumePath   string `json:"volume_path" validate:"volume_path|required|regex:^/"`
	VolumeName   string `json:"volume_name" validate:"volume_name|required|max:50"`
}

// RestorePluginsReq defines a struct to receive the request body
// to restore service dependent volumes.
type RestorePluginsReq struct {
	Plugins []*RestorePlugin `validate:"plugins|required" json:"plugins"`
}

// RestorePlugin holds infomations of service dependenct volume.
type RestorePlugin struct {
	PluginID  string `json:"plugin_id" validate:"plugin_id"`
	VersionID string `json:"version_id" validate:"version_id"`
	Switch    bool   `json:"switch" validate:"switch|bool"`
}
