package model

// DeleteRegistryImageManifestRequest deletes one internal registry image manifest.
type DeleteRegistryImageManifestRequest struct {
	Image string `json:"image" validate:"required"`
}
