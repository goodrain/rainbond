package cleanup

import (
	"strings"

	"github.com/docker/distribution/reference"
)

// ReferenceScopesForImage returns repository dependencies without guessing an
// implicit registry. A nil result requires conservative conflict checking.
func ReferenceScopesForImage(image string) []string {
	var scopes []string
	// An unqualified image can be resolved differently by the runtime and the
	// platform. Do not infer a Docker Hub library scope for a local dependency.
	first, _, qualified := strings.Cut(image, "/")
	if !qualified || (!strings.ContainsAny(first, ".:") && first != "localhost") {
		return nil
	}
	if named, err := reference.ParseNormalizedNamed(image); err == nil {
		scopes = []string{reference.Path(named)}
	}
	return scopes
}

func versionRowReferenceScopes(record versionRow) []string {
	image := record.ImageName
	if image == "" && record.DeliveredType == "image" {
		image = record.DeliveredPath
	}
	return ReferenceScopesForImage(image)
}
