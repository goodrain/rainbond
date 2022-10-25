package model

import "github.com/goodrain/rainbond/db/model"

// HistoryLogFile represents a history log file for service
type HistoryLogFile struct {
	Filename     string `json:"filename"`
	RelativePath string `json:"relative_path"`
}

// MyTeamsEventsReq -
type MyTeamsEventsReq struct {
	TenantIDs []string `json:"tenant_ids"`
}

// MyTeamsEvent -
type MyTeamsEvent struct {
	ServiceEvent   *model.ServiceEvent `json:"service_event"`
	BuildList 	   *BuildListRespVO `json:"build_list"`
}
