package handler

import (
	apimodel "github.com/goodrain/rainbond/api/model"
)

// AppRestoreHandler defines handler methods to restore app.
// app means market service.
type AppRestoreHandler interface {
	RestoreEnvs(tenantID, serviceID string, req *apimodel.RestoreEnvsReq) error
	RestorePorts(tenantID, serviceID string, req *apimodel.RestorePortsReq) error
	RestoreVolumes(tenantID, serviceID string, req *apimodel.RestoreVolumesReq) error
	RestoreProbe(serviceID string, req *apimodel.ServiceProbe) error
	RestoreDeps(tenantID, serviceID string, req *apimodel.RestoreDepsReq) error
	RestoreDepVols(tenantID, serviceID string, req *apimodel.RestoreDepVolsReq) error
	RestorePlugins(tenantID, serviceID string, req *apimodel.RestorePluginsReq) error
}

// NewAppRestoreHandler creates a new AppRestoreHandler.
func NewAppRestoreHandler() AppRestoreHandler {
	return &AppRestoreAction{}
}
