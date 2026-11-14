package vmrestore

import "context"

// UnqualifiedRestorePreparation is the explicit production boundary for
// missing trusted metrics/certification and native bootstrap provenance. It
// refuses before preparing a fence or allocating any resource.
type UnqualifiedRestorePreparation struct{}

// Read cannot manufacture a trusted source from user-provided identifiers.
func (UnqualifiedRestorePreparation) Read(context.Context, Scope, string, string, string) (RestorePreparation, error) {
	return RestorePreparation{}, ErrRestoreCapability
}
