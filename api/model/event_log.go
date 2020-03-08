package model

// HistoryLogFile represents a history log file for service
type HistoryLogFile struct {
	Filename     string `json:"filename"`
	RelativePath string `json:"relative_path"`
}
