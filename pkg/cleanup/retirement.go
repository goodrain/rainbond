// Package cleanup contains guards shared by version retirement and activation.
package cleanup

import "errors"

var ErrStateChanged = errors.New("version state changed; rescan required")
var ErrVersionProtected = errors.New("version is active or unsupported for retirement")

type VersionExpectation struct {
	Operator           string `json:"operator,omitempty"`
	OperationID        string `json:"operation_id,omitempty"`
	ActivationRevision string `json:"activation_revision"`
	ServiceID          string `json:"service_id"`
	EventID            string `json:"event_id"`
	Version            string `json:"version"`
	CurrentVersion     string `json:"current_version"`
	Image              string `json:"image"`
}

type VersionState struct {
	ActivationRevision                                                        string
	ServiceID, Version, CurrentVersion, Image, Status, DeliveredType, EventID string
	ActiveOperation                                                           bool
}

// ValidateVersionRetirement is for metadata retirement, not registry deletion.
// An immutable identity and a fresh current-version check are mandatory.
func ValidateVersionRetirement(state VersionState, expected VersionExpectation) error {
	if state.ActivationRevision != expected.ActivationRevision || expected.EventID == "" || state.EventID != expected.EventID || expected.ServiceID == "" || expected.Version == "" || expected.CurrentVersion == "" || expected.Image == "" ||
		state.ServiceID != expected.ServiceID || state.Version != expected.Version || state.Image != expected.Image || state.CurrentVersion != expected.CurrentVersion {
		return ErrStateChanged
	}
	if state.Version == state.CurrentVersion || state.ActiveOperation || state.DeliveredType != "image" || (state.Status != "success" && state.Status != "failure") {
		return ErrVersionProtected
	}
	return nil
}
