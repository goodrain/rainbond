package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources/registry"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type scheduledVersionRegistry interface {
	Resolve(context.Context, string, string) (string, error)
	Tags(context.Context, string) ([]string, error)
	Delete(context.Context, string, string, string) (int, error)
	Exists(context.Context, string, string) (bool, error)
}

// runScheduledVersionCleanup uses the installed system ingress only. Merely
// knowing a registry URL must not authorize cleanup of another registry.
func (h *CleanupCoordinationHandler) runScheduledVersionCleanup(ctx context.Context, binding guard.StorageRegistration, triggerID string, keepCount uint) error {
	if h == nil || h.gcTarget == nil {
		return guard.ErrCoordinationUnavailable
	}
	client, namespace, serviceName, err := h.gcTarget()
	if err != nil || client == nil {
		return guard.ErrCoordinationUnavailable
	}
	endpoint := configs.Default().ServerConfig.RbdHub
	origin, err := url.Parse(endpoint)
	if err != nil || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") || (origin.Scheme != "http" && origin.Scheme != "https") {
		return guard.ErrCoordinationUnavailable
	}
	host := strings.ToLower(origin.Hostname())
	if host != serviceName && host != serviceName+"."+namespace && host != serviceName+"."+namespace+".svc" && host != serviceName+"."+namespace+".svc.cluster.local" {
		return guard.ErrCoordinationUnavailable
	}
	service, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil || service.UID == "" || service.DeletionTimestamp != nil {
		return guard.ErrCoordinationUnavailable
	}
	port := 80
	if origin.Scheme == "https" {
		port = 443
	}
	if origin.Port() != "" {
		port, err = strconv.Atoi(origin.Port())
		if err != nil {
			return guard.ErrCoordinationUnavailable
		}
	}
	matched := false
	for _, p := range service.Spec.Ports {
		matched = matched || int(p.Port) == port
	}
	if !matched {
		return guard.ErrCoordinationUnavailable
	}
	remote, err := registry.NewCoordinatedCleanupClient(endpoint, builder.REGISTRYUSER, builder.REGISTRYPASS)
	if err != nil {
		return guard.ErrCoordinationUnavailable
	}
	return h.runScheduledVersionCleanupWithRegistry(ctx, binding, triggerID, keepCount, remote, origin.Host)
}

func scheduledVersionImage(image, managedHost string) (repository, tag string, ok bool) {
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return "", "", false
	}
	domain := reference.Domain(named)
	if domain != "goodrain.me" && domain != managedHost {
		return "", "", false
	}
	tagged, yes := named.(reference.Tagged)
	if !yes {
		return "", "", false
	}
	if _, digested := named.(reference.Digested); digested {
		return "", "", false
	}
	repository = reference.Path(named)
	// Preserve the legacy protection for builder and runner system images.
	if strings.Contains(repository, "builder") || strings.Contains(repository, "runner") {
		return "", "", false
	}
	return repository, tagged.Tag(), true
}

const scheduledVersionCleanupOwner = "chaos-scheduled-version-cleanup"

type scheduledVersionManifestGroup struct {
	Repository, Target string
	Versions           []guard.ScheduledVersionRetirement
}

func (h *CleanupCoordinationHandler) runScheduledVersionCleanupWithRegistry(ctx context.Context, binding guard.StorageRegistration, triggerID string, keepCount uint, remote scheduledVersionRegistry, managedHost string) error {
	if h == nil || h.database == nil || remote == nil || keepCount < 1 || keepCount > 10000 || triggerID == "" || len(triggerID) > 128 || !guard.IsRegistryStorageBinding(binding) {
		return guard.ErrCoordinationChanged
	}
	if err := h.recoverUnstartedScheduledVersionCandidates(ctx, binding, remote); err != nil {
		return err
	}
	if observation, ready, err := h.certifyRegistry(ctx, binding, nil); err != nil || !ready {
		if err == nil && observation.Mode == "ready" {
			var uncertain int
			if err := h.database().Model(&model.CleanupOperation{}).Where("storage_id = ? AND (state = ? OR (owner IN (?) AND kind = ? AND state IN (?)))", binding.StorageID, "uncertain", []string{scheduledVersionCleanupOwner, "chaos-scheduled-version-retry"}, "delete", []string{"executing", "applied", "rejected"}).Count(&uncertain).Error; err != nil {
				return err
			}
			if uncertain > 0 {
				return guard.ErrCoordinationUncertain
			}
			return guard.ErrCoordinationBusy
		}
		return guard.ErrCoordinationUnavailable
	}
	database := h.database()
	// The original receipt remains discoverable even after its version records
	// were retired and a later read or verification could not complete.
	var pending []model.CleanupOperation
	if err := database.Where("storage_id = ? AND generation = ? AND owner = ? AND kind = ? AND state = ? AND outcome IN (?)", binding.StorageID, binding.Generation, scheduledVersionCleanupOwner, "delete", "finished", []string{guard.ScheduledVersionRetirementPending, "rejected"}).Order("operation_id").Limit(1001).Find(&pending).Error; err != nil {
		return err
	}
	if len(pending) > 1000 {
		return guard.ErrCoordinationUnavailable
	}
	for _, candidate := range pending {
		group := scheduledVersionManifestGroup{Repository: candidate.Scope, Target: candidate.Target}
		if err := h.deleteScheduledVersionManifest(ctx, binding, triggerID, group, keepCount, remote, &candidate); err != nil {
			return err
		}
	}
	var counts []model.VersionInfoCount
	if err := database.Model(&model.VersionInfo{}).Select("service_id, COUNT(*) AS count").Where("final_status = ?", "success").Group("service_id").Having("COUNT(*) > ?", keepCount).Limit(1001).Scan(&counts).Error; err != nil {
		return err
	}
	if len(counts) > 1000 {
		return guard.ErrCoordinationUnavailable
	}
	sort.Slice(counts, func(i, j int) bool { return counts[i].ServiceID < counts[j].ServiceID })
	groups := map[string]*scheduledVersionManifestGroup{}
	for _, count := range counts {
		if err := ctx.Err(); err != nil {
			return err
		}
		var services []model.TenantServices
		if err := database.Where("service_id = ?", count.ServiceID).Limit(2).Find(&services).Error; err != nil {
			return err
		}
		if len(services) != 1 || services[0].TenantID == "" || services[0].DeployVersion == "" {
			continue
		}
		service := services[0]
		var versions []model.VersionInfo
		if err := database.Where("service_id = ? AND final_status = ?", count.ServiceID, "success").Order("ID DESC").Limit(10001).Find(&versions).Error; err != nil {
			return err
		}
		if len(versions) > 10000 {
			return guard.ErrCoordinationUnavailable
		}
		if len(versions) <= int(keepCount) {
			continue
		}
		for _, version := range versions[int(keepCount):] {
			if err := ctx.Err(); err != nil {
				return err
			}
			if version.DeliveredType != "image" || version.BuildVersion == service.DeployVersion {
				continue
			}
			image := version.ImageName
			if image == "" {
				image = version.DeliveredPath
			}
			delivered := version.DeliveredPath
			if delivered == "" {
				delivered = image
			}
			repository, _, ok := scheduledVersionImage(delivered, managedHost)
			if !ok {
				continue
			}
			if version.ImageName != "" && version.DeliveredPath != "" {
				a, e1 := reference.ParseNormalizedNamed(image)
				b, e2 := reference.ParseNormalizedNamed(delivered)
				if e1 != nil || e2 != nil || a.String() != b.String() {
					continue
				}
			}
			_, tag, ok := scheduledVersionImage(image, managedHost)
			if !ok {
				continue
			}
			target, err := remote.Resolve(ctx, repository, tag)
			if err != nil {
				continue
			}
			key := repository + "\x00" + target
			if groups[key] == nil {
				groups[key] = &scheduledVersionManifestGroup{Repository: repository, Target: target}
			}
			expected := guard.VersionExpectation{ServiceID: version.ServiceID, Version: version.BuildVersion, CurrentVersion: service.DeployVersion, Image: image, EventID: version.EventID, ActivationRevision: version.ActivationRevision}
			groups[key].Versions = append(groups[key].Versions, guard.ScheduledVersionRetirement{TenantID: service.TenantID, Expected: expected})
			if len(groups[key].Versions) > 1000 || len(groups) > 1000 {
				return guard.ErrCoordinationUnavailable
			}
		}
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := h.deleteScheduledVersionManifest(ctx, binding, triggerID, *groups[key], keepCount, remote, nil); err != nil {
			return err
		}
	}
	return nil
}

func (h *CleanupCoordinationHandler) deleteScheduledVersionManifest(ctx context.Context, binding guard.StorageRegistration, triggerID string, group scheduledVersionManifestGroup, keepCount uint, remote scheduledVersionRegistry, pending *model.CleanupOperation) (resultErr error) {
	repository, target := group.Repository, group.Target
	selection := repository + "\x00" + target
	for _, version := range group.Versions {
		selection += "\x00" + version.TenantID + "\x00" + version.Expected.ServiceID + "\x00" + version.Expected.Version + "\x00" + version.Expected.EventID + "\x00" + version.Expected.ActivationRevision
	}
	if pending != nil {
		selection += "\x00pending:" + pending.OperationID
	}
	sum := sha256.Sum256([]byte(triggerID + "\x00" + binding.StorageID + "\x00" + binding.Generation + "\x00" + selection))
	identity := hex.EncodeToString(sum[:])
	request := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, Owner: scheduledVersionCleanupOwner, OperationID: identity, Kind: "delete", Scope: repository, Target: target, Fingerprint: identity}
	if pending != nil {
		request.Owner = "chaos-scheduled-version-retry"
	}
	created, err := guard.AcquireOperation(h.database(), request)
	if err != nil {
		return err
	}
	if !created {
		return nil
	}
	confirmed := true
	defer func() {
		if err := guard.FinishOperation(h.database(), request, confirmed); resultErr == nil {
			resultErr = err
		}
	}()
	if pending != nil {
		exists, err := remote.Exists(ctx, repository, target)
		if err != nil {
			return err
		}
		if !exists {
			return h.finishScheduledVersionCandidate(*pending)
		}
	}
	tags, err := remote.Tags(ctx, repository)
	if err != nil {
		return err
	}
	sort.Strings(tags)
	matching := []string{}
	observed := map[string]string{}
	for _, alias := range tags {
		digest, err := remote.Resolve(ctx, repository, alias)
		if err != nil {
			return err
		}
		observed[alias] = digest
		if digest == target {
			matching = append(matching, alias)
		}
	}
	for _, candidate := range group.Versions {
		named, err := reference.ParseNormalizedNamed(candidate.Expected.Image)
		if err != nil {
			return guard.ErrCoordinationChanged
		}
		tagged, ok := named.(reference.Tagged)
		if !ok || observed[tagged.Tag()] != target {
			return nil
		}
	}
	keepStart := len(tags) - int(keepCount)
	if keepStart < 0 {
		keepStart = 0
	}
	for _, alias := range tags[keepStart:] {
		if observed[alias] == target {
			return nil
		}
	}
	if h.clusterReferences == nil || h.permitKey == nil {
		return guard.ErrCoordinationUnavailable
	}
	references, err := h.clusterReferences(ctx)
	if err != nil {
		return guard.ErrCoordinationUnavailable
	}
	if err := h.requireReferenceWriterCoverage(ctx); err != nil {
		return err
	}
	if _, ready, err := h.certifyRegistry(ctx, binding, &request); err != nil || !ready {
		return guard.ErrCoordinationUnavailable
	}
	if pending == nil {
		err := guard.RetireScheduledVersions(h.database(), request, group.Versions, keepCount, matching, references)
		if errors.Is(err, guard.ErrVersionProtected) || errors.Is(err, guard.ErrStateChanged) || gorm.IsRecordNotFoundError(err) {
			return nil
		}
		if err != nil {
			return err
		}
	} else {
		audit, err := guard.AuditRegionManifestReferences(h.database(), request, matching, references)
		if err != nil {
			return err
		}
		if !audit.Complete {
			return guard.ErrCoordinationUnavailable
		}
		if audit.Referenced {
			return nil
		}
	}
	// This revalidation follows the atomic retirement. A failed revalidation
	// retains retired-pending, rather than losing the physical cleanup candidate.
	if _, ready, err := h.certifyRegistry(ctx, binding, &request); err != nil || !ready {
		return guard.ErrCoordinationUnavailable
	}
	if digest, err := remote.Resolve(ctx, repository, target); err != nil || digest != target {
		return guard.ErrCoordinationChanged
	}
	permit, err := guard.IssueRegistryDeletionPermit(h.permitKey(), request, time.Now())
	if err != nil {
		return guard.ErrCoordinationUnavailable
	}
	confirmed = false
	status, err := remote.Delete(ctx, repository, target, permit)
	state, stateErr := h.waitScheduledDeletionOutcome(ctx, request)
	if err != nil || stateErr != nil {
		return guard.ErrCoordinationUncertain
	}
	if status != http.StatusAccepted {
		if state == "rejected" {
			confirmed = true
			return guard.ErrCoordinationDenied
		}
		return guard.ErrCoordinationUncertain
	}
	if state != "applied" {
		return guard.ErrCoordinationUncertain
	}
	exists, err := remote.Exists(ctx, repository, target)
	if err != nil || exists {
		return guard.ErrCoordinationUncertain
	}
	confirmed = true
	if pending != nil {
		return h.finishScheduledVersionCandidate(*pending)
	}
	return nil
}

func (h *CleanupCoordinationHandler) finishScheduledVersionCandidate(candidate model.CleanupOperation) error {
	updated := h.database().Model(&model.CleanupOperation{}).Where("operation_id = ? AND storage_id = ? AND generation = ? AND owner = ? AND kind = ? AND scope = ? AND target = ? AND fingerprint = ? AND state = ? AND outcome = ?", candidate.OperationID, candidate.StorageID, candidate.Generation, scheduledVersionCleanupOwner, "delete", candidate.Scope, candidate.Target, candidate.Fingerprint, "finished", candidate.Outcome).UpdateColumn("outcome", "applied")
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected == 1 {
		return nil
	}
	var current model.CleanupOperation
	if err := h.database().Where("operation_id = ?", candidate.OperationID).First(&current).Error; err != nil {
		return err
	}
	if current.StorageID == candidate.StorageID && current.Generation == candidate.Generation && current.Owner == scheduledVersionCleanupOwner && current.Kind == "delete" && current.Scope == candidate.Scope && current.Target == candidate.Target && current.Fingerprint == candidate.Fingerprint && current.State == "finished" && current.Outcome == "applied" {
		return nil
	}
	return guard.ErrCoordinationChanged
}

// The proxy persists its result after forwarding the Registry response. Observe
// that receipt briefly without ever replaying the destructive HTTP request.
func (h *CleanupCoordinationHandler) waitScheduledDeletionOutcome(ctx context.Context, request guard.CoordinationRequest) (string, error) {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := guard.InspectOperation(h.database(), request)
		if err != nil || (state != "active" && state != "executing") {
			return state, err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return "", guard.ErrCoordinationUncertain
		case <-ticker.C:
		}
	}
}

// Recovery precedes readiness certification: an interrupted caller may have
// committed retirement but crashed before its normal Finish defer could run.
func (h *CleanupCoordinationHandler) recoverUnstartedScheduledVersionCandidates(ctx context.Context, binding guard.StorageRegistration, remote scheduledVersionRegistry) error {
	var candidates []model.CleanupOperation
	owners := []string{scheduledVersionCleanupOwner, "chaos-scheduled-version-retry"}
	query := "storage_id = ? AND generation = ? AND owner IN (?) AND kind = ? AND ((state = ? AND ((owner = ? AND outcome = ?) OR (owner = ? AND outcome = ?))) OR state IN (?))"
	if err := h.database().Where(query, binding.StorageID, binding.Generation, owners, "delete", "active", scheduledVersionCleanupOwner, guard.ScheduledVersionRetirementPending, "chaos-scheduled-version-retry", "", []string{"applied", "rejected"}).Order("operation_id").Limit(1001).Find(&candidates).Error; err != nil {
		return err
	}
	if len(candidates) > 1000 {
		return guard.ErrCoordinationUnavailable
	}
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return err
		}
		request := guard.CoordinationRequest{StorageID: candidate.StorageID, Generation: candidate.Generation, Owner: candidate.Owner, OperationID: candidate.OperationID, Kind: candidate.Kind, Scope: candidate.Scope, Target: candidate.Target, Fingerprint: candidate.Fingerprint}
		if candidate.State == "active" {
			if err := guard.RecoverUnstartedScheduledVersionDeletion(h.database(), request); err != nil {
				return err
			}
			continue
		}
		switch candidate.State {
		case "rejected":
			// The trusted proxy persisted a definitive rejection, so no DELETE effect
			// is being guessed. The primary candidate stays available for a later run.
			if candidate.Outcome != "rejected" {
				return guard.ErrCoordinationChanged
			}
			if err := guard.FinishOperation(h.database(), request, true); err != nil {
				return err
			}
		case "applied":
			if candidate.Outcome != "applied" {
				return guard.ErrCoordinationChanged
			}
			exists, err := remote.Exists(ctx, candidate.Scope, candidate.Target)
			if err != nil || exists {
				return guard.ErrCoordinationUncertain
			}
			// Capture only pre-existing primary candidates while this original scope
			// is still held; never clear a newer candidate after releasing admission.
			var originals []model.CleanupOperation
			if err := h.database().Where("storage_id = ? AND generation = ? AND owner = ? AND kind = ? AND scope = ? AND target = ? AND state = ? AND outcome IN (?)", binding.StorageID, binding.Generation, scheduledVersionCleanupOwner, "delete", candidate.Scope, candidate.Target, "finished", []string{guard.ScheduledVersionRetirementPending, "rejected"}).Limit(1001).Find(&originals).Error; err != nil {
				return err
			}
			if len(originals) > 1000 {
				return guard.ErrCoordinationUnavailable
			}
			if err := guard.FinishOperation(h.database(), request, true); err != nil {
				return err
			}
			for _, original := range originals {
				if err := h.finishScheduledVersionCandidate(original); err != nil {
					return err
				}
			}
		default:
			return guard.ErrCoordinationUncertain
		}
	}
	return nil
}
