package cleanup

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"sort"

	"github.com/docker/distribution/reference"
)

// InspectHelmReleaseReferences reads only rendered manifests and hooks from a
// retained release. Values, chart files and credentials are never returned.
// Corruption, unsupported workloads and limits are incomplete evidence, not an
// empty proof. Both current gzip and legacy uncompressed Helm encodings work.
func InspectHelmReleaseReferences(encoded, name, namespace string, revision int) RegionReferenceInventory {
	denied := RegionReferenceInventory{}
	if len(encoded) == 0 || len(encoded) > 1<<20 || name == "" || namespace == "" || revision < 1 {
		return denied
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return denied
	}
	if len(raw) >= 3 && bytes.Equal(raw[:3], []byte{0x1f, 0x8b, 0x08}) {
		reader, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return denied
		}
		raw, err = io.ReadAll(io.LimitReader(reader, (8<<20)+1))
		reader.Close()
		if err != nil || len(raw) > 8<<20 {
			return denied
		}
	}
	var release struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Version   int    `json:"version"`
		Manifest  string `json:"manifest"`
		Hooks     []struct {
			Manifest string `json:"manifest"`
		} `json:"hooks"`
	}
	if json.Unmarshal(raw, &release) != nil || release.Name != name || release.Namespace != namespace || release.Version != revision || len(release.Hooks) > 1000 {
		return denied
	}
	result := RegionReferenceInventory{Complete: true, Images: []string{}}
	images := map[string]bool{}
	record := func(image string) {
		if _, err := reference.ParseNormalizedNamed(image); err != nil {
			result.Complete = false
			return
		}
		if len(images) >= 20000 && !images[image] {
			result.Complete = false
			return
		}
		images[image] = true
	}
	if release.Manifest != "" && !inspectSavedWorkload(release.Manifest, record) {
		result.Complete = false
	}
	for _, hook := range release.Hooks {
		if !inspectSavedWorkload(hook.Manifest, record) {
			result.Complete = false
		}
	}
	for image := range images {
		result.Images = append(result.Images, image)
	}
	sort.Strings(result.Images)
	return result
}
