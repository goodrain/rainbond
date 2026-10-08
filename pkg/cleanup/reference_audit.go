package cleanup

import (
	"regexp"
	"sort"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// RegionReferenceAudit covers retained Region records. Console templates,
// snapshots and live workloads still require checks under the same admission.
type RegionReferenceAudit struct {
	Complete   bool `json:"region_records_complete"`
	Referenced bool `json:"referenced"`
}

// HelmReleaseIdentity is internal correlation evidence, never a public inventory field.
type HelmReleaseIdentity struct {
	ValueHashes                                  map[string]string
	Name, Namespace, Chart, ChartVersion, Status string
	Revision                                     int
}

// RegionReferenceInventory contains image identities, never raw saved configs.
type RegionReferenceInventory struct {
	HelmReleases []HelmReleaseIdentity `json:"-"`
	Complete     bool                  `json:"region_records_complete"`
	Images       []string              `json:"images"`
}

var referenceAuditTag = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

const kubeBlocksBuildKind = "kubeblocks"

func collectRegionReferenceImages(tx *gorm.DB) (RegionReferenceInventory, error) {
	denied := RegionReferenceInventory{}
	result := RegionReferenceInventory{Complete: true, Images: []string{}}
	images := map[string]bool{}
	overflow := false
	inspect := func(image string) {
		if image == "" {
			return
		}
		if _, err := reference.ParseNormalizedNamed(image); err != nil {
			result.Complete = false
			return
		}
		if len(images) >= 20000 && !images[image] {
			overflow = true
			return
		}
		images[image] = true
	}
	var versions []model.VersionInfo
	if err := tx.Select("image_name, delivered_type, delivered_path, final_status, kind").Limit(20001).Find(&versions).Error; err != nil {
		return denied, err
	}
	if len(versions) > 20000 {
		return denied, ErrCoordinationUnavailable
	}
	for _, version := range versions {
		inspect(version.ImageName)
		if version.DeliveredType == "image" {
			inspect(version.DeliveredPath)
		}
		// KubeBlocks build events only trigger custom-resource deployment and do
		// not produce a container image. Their empty image fields are complete
		// evidence, not an unknown Registry reference.
		if version.FinalStatus == "success" && version.Kind != kubeBlocksBuildKind && version.ImageName == "" && version.DeliveredType != "slug" && (version.DeliveredType != "image" || version.DeliveredPath == "") {
			result.Complete = false
		}
	}
	var plugins []model.TenantPluginBuildVersion
	if err := tx.Select("base_image, build_local_image, status").Limit(20001).Find(&plugins).Error; err != nil {
		return denied, err
	}
	if len(plugins) > 20000 {
		return denied, ErrCoordinationUnavailable
	}
	for _, plugin := range plugins {
		inspect(plugin.BaseImage)
		inspect(plugin.BuildLocalImage)
		if plugin.Status == "complete" && plugin.BuildLocalImage == "" {
			result.Complete = false
		}
	}
	// Stream bounded documents rather than loading every saved manifest at once.
	rows, err := tx.Model(&model.K8sResource{}).Select("kind, CASE WHEN LENGTH(content) <= 1048576 THEN content ELSE '' END").Limit(20001).Rows()
	if err != nil {
		return denied, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		if count > 20000 {
			return denied, ErrCoordinationUnavailable
		}
		var kind, content string
		if err := rows.Scan(&kind, &content); err != nil {
			return denied, err
		}
		// CRD schemas describe APIs rather than runnable Pod templates. Large
		// KubeBlocks CRDs are not Registry references and must not turn a
		// complete workload audit into unknown coverage.
		if kind == "CustomResourceDefinition" {
			continue
		}
		if !inspectSavedWorkload(content, inspect) {
			result.Complete = false
		}
	}
	if err := rows.Err(); err != nil {
		return denied, err
	}
	if err := rows.Close(); err != nil {
		return denied, err
	}
	importComplete, err := inspectImportReferences(tx, inspect)
	if err != nil {
		return denied, err
	}
	result.Complete = result.Complete && importComplete
	if overflow {
		return denied, ErrCoordinationUnavailable
	}
	for image := range images {
		result.Images = append(result.Images, image)
	}
	sort.Strings(result.Images)
	return result, nil
}

// ReadRegionReferenceInventory is advisory scan evidence. It never grants a
// deletion; execution must repeat the audit after acquiring the exact scope.
func ReadRegionReferenceInventory(database *gorm.DB, storage, generation string) (RegionReferenceInventory, error) {
	denied := RegionReferenceInventory{}
	if !coordinationIdentity.MatchString(storage) || !coordinationIdentity.MatchString(generation) {
		return denied, ErrCoordinationChanged
	}
	tx := database.Begin()
	if tx.Error != nil {
		return denied, tx.Error
	}
	defer tx.Rollback()
	store, err := lockCleanupStorage(tx, CoordinationRequest{StorageID: storage, Generation: generation})
	if err != nil {
		return denied, err
	}
	// Collecting must expose advisory references for initial coverage assessment.
	// This read never promotes readiness or creates a deletion admission.
	if store.Mode != "ready" && store.Mode != "collecting" {
		return denied, ErrCoordinationBusy
	}
	result, err := collectRegionReferenceImages(tx)
	if err != nil {
		return denied, err
	}
	if err := tx.Commit().Error; err != nil {
		return denied, err
	}
	return result, nil
}

// AuditRegionManifestReferences checks retained records under the original active deletion admission.
func AuditRegionManifestReferences(database *gorm.DB, r CoordinationRequest, tags []string, extra ...RegionReferenceInventory) (RegionReferenceAudit, error) {
	denied := RegionReferenceAudit{}
	if !r.valid() || r.Kind != "delete" || !registryDeletionDigest.MatchString(r.Target) || len(tags) > 256 {
		return denied, ErrCoordinationChanged
	}
	selectedTags := map[string]bool{}
	for _, tag := range tags {
		if !referenceAuditTag.MatchString(tag) {
			return denied, ErrCoordinationChanged
		}
		selectedTags[tag] = true
	}
	tx := database.Begin()
	if tx.Error != nil {
		return denied, tx.Error
	}
	defer tx.Rollback()
	if _, err := lockCleanupStorage(tx, r); err != nil {
		return denied, err
	}
	var operation model.CleanupOperation
	if err := tx.Where("operation_id = ?", r.OperationID).First(&operation).Error; err != nil {
		return denied, err
	}
	if !r.matches(operation) || operation.State != "active" {
		return denied, ErrCoordinationChanged
	}
	inventory, err := collectRegionReferenceImages(tx)
	if err != nil {
		return denied, err
	}
	inventory = MergeReferenceInventories(append([]RegionReferenceInventory{inventory}, extra...)...)
	result := RegionReferenceAudit{Complete: inventory.Complete}
	for _, image := range inventory.Images {
		named, err := reference.ParseNormalizedNamed(image)
		if err != nil {
			return denied, ErrCoordinationChanged
		}
		if digest, ok := named.(reference.Digested); ok {
			if digest.Digest().String() == r.Target {
				result.Referenced = true
			}
			continue
		}
		tagged, ok := reference.TagNameOnly(named).(reference.Tagged)
		if !ok {
			return denied, ErrCoordinationChanged
		}
		repository := reference.Path(named)
		if (repository == r.Scope || strings.TrimPrefix(repository, "library/") == r.Scope) && selectedTags[tagged.Tag()] {
			result.Referenced = true
		}
	}
	if err := tx.Commit().Error; err != nil {
		return denied, err
	}
	return result, nil
}

// MergeReferenceInventories preserves incompleteness and returns only validated,
// deduplicated image identities. It never grants deletion or changes store state.
func MergeReferenceInventories(inventories ...RegionReferenceInventory) RegionReferenceInventory {
	result := RegionReferenceInventory{Complete: len(inventories) > 0, Images: []string{}}
	images := map[string]bool{}
	for _, inventory := range inventories {
		result.Complete = result.Complete && inventory.Complete
		for _, image := range inventory.Images {
			if _, err := reference.ParseNormalizedNamed(image); err != nil {
				result.Complete = false
				continue
			}
			if len(images) >= 20000 && !images[image] {
				result.Complete = false
				continue
			}
			images[image] = true
		}
	}
	for image := range images {
		result.Images = append(result.Images, image)
	}
	sort.Strings(result.Images)
	return result
}

// ConfiguredImageReferences protects explicitly configured future job images.
// Empty values mean the optional executor is disabled, not an unknown image.
func ConfiguredImageReferences(values []string) RegionReferenceInventory {
	inventory := RegionReferenceInventory{Complete: true, Images: []string{}}
	for _, value := range values {
		if value != "" {
			inventory.Images = append(inventory.Images, value)
		}
	}
	return MergeReferenceInventories(inventory)
}
