package controller

import (
	"encoding/json"
	"net/http"
	"os"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	httputil "github.com/goodrain/rainbond/util/http"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

func systemNodeInventorySettings() kubeidentity.NodeInventorySettings {
	return kubeidentity.NodeInventorySettings{Region: os.Getenv("REGION_NAME"), Image: os.Getenv("CLEANUP_NODE_EXECUTOR_IMAGE"), IncludePackages: os.Getenv("CLEANUP_NODE_PACKAGE_INVENTORY") == "true"}
}

// CollectManagedCache starts an authenticated, manually requested inventory job.
// Paths, images and storage names come from trusted installation configuration.
func (h *CleanupCoordinationHandler) CollectManagedCache(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pod    string `json:"pod"`
		PodUID string `json:"pod_uid"`
		ScanID string `json:"scan_id"`
	}
	if !coordinationDecode(w, r, &body) {
		return
	}
	if len(validation.IsDNS1123Subdomain(body.Pod)) != 0 || !registryPodUID.MatchString(body.PodUID) || !registryPodUID.MatchString(body.ScanID) {
		httputil.ReturnError(r, w, 400, "INVALID_MANAGED_CACHE_SCAN")
		return
	}
	if h.gcTarget == nil || h.inventorySettings == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	client, namespace, _, err := h.gcTarget()
	if err != nil || client == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	observed, err := kubeidentity.InspectManagedBuildCacheSource(r.Context(), client, namespace, body.Pod, body.PodUID)
	if err != nil {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	binding, err := guard.ProvisionManagedCacheStorage(h.database(), observed.Mount.VolumeUID, observed.Root)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	settings := h.inventorySettings()
	settings.ScanID = body.ScanID
	fresh := func() (*batchv1.Job, error) {
		return kubeidentity.BuildManagedCacheInventoryJob(r.Context(), client, namespace, body.Pod, body.PodUID, binding, settings)
	}
	template, err := fresh()
	if err != nil {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	job, err := guard.RunNodeInventoryJob(r.Context(), client.BatchV1().Jobs(namespace), body.ScanID, template, fresh)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	state := "submitted"
	var report json.RawMessage
	if job.Status.Succeeded > 0 {
		state = "succeeded"
		report, err = kubeidentity.ReadNodeInventoryReport(r.Context(), client, job, binding, settings)
		if err != nil {
			coordinationError(w, r, guard.ErrCoordinationUnavailable)
			return
		}
	} else if job.Status.Failed > 0 {
		state = "failed"
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol   int             `json:"protocol"`
		State      string          `json:"state"`
		JobName    string          `json:"job_name"`
		JobUID     string          `json:"job_uid"`
		StorageID  string          `json:"storage_id"`
		NodeUID    string          `json:"node_uid"`
		Generation string          `json:"generation"`
		Report     json.RawMessage `json:"report,omitempty"`
	}{1, state, job.Name, string(job.UID), binding.StorageID, observed.NodeUID, binding.Generation, report})
}
