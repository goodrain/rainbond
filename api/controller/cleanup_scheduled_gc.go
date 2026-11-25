package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	"github.com/jinzhu/gorm"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

var errScheduledGCFailed = errors.New("scheduled Registry GC did not succeed")

type scheduledVersionCleanup func(context.Context, guard.StorageRegistration, string, uint) error

// RunScheduledCleanup runs only the enrolled system Registry's coordinated
// cleanup protocol. It never enables legacy cleaners or falls back to Pod exec.
// triggerID is shared across replicas: startup means this Chaos DaemonSet
// rollout; daily triggers contain the scheduler's original RFC3339 time slot.
func (h *CleanupCoordinationHandler) RunScheduledCleanup(ctx context.Context, triggerID string, keepCount uint) error {
	return h.runScheduledCleanup(ctx, triggerID, keepCount, h.runScheduledVersionCleanup)
}

func (h *CleanupCoordinationHandler) runScheduledCleanup(ctx context.Context, triggerID string, keepCount uint, cleanupVersions scheduledVersionCleanup) (result error) {
	if ctx == nil || h == nil || h.database == nil || h.gcTarget == nil || h.inspectRegistry == nil || cleanupVersions == nil || keepCount == 0 {
		return guard.ErrCoordinationUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	database := h.database()
	if database == nil {
		return guard.ErrCoordinationUnavailable
	}
	slot, err := h.scheduledGCTrigger(ctx, triggerID)
	if err != nil {
		return err
	}
	client, namespace, service, err := h.gcTarget()
	if err != nil || client == nil || namespace == "" || service == "" {
		return guard.ErrCoordinationUnavailable
	}
	binding, err := h.scheduledRegistryBinding(ctx, client, namespace, service)
	if err != nil {
		return err
	}
	request := scheduledGCRequest(binding, slot, keepCount)
	work := scheduledGCWork{database: database, client: client, namespace: namespace, service: service, binding: binding, request: request}
	// This checks every selected ingress, pinned helper, physical mount and the
	// explicit disabled legacy cleanup flag before even retiring a build record.
	template, err := work.currentTemplate(ctx)
	if err != nil {
		return err
	}
	progress, err := guard.ReadGCJobProgress(database, request)
	if err == nil {
		if progress.State == "finished" {
			return scheduledGCResult(progress)
		}
	} else {
		if !gorm.IsRecordNotFoundError(err) {
			return err
		}
		if err := cleanupVersions(ctx, binding, slot, keepCount); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := guard.RequestIdleMaintenance(database, request); err != nil {
		return err
	}
	defer func() {
		if result == nil || ctx.Err() == nil {
			return
		}
		// Caller cancellation cannot make an already granted/ambiguous executor
		// writable. A short independent context may cancel a verified idle drain.
		cancelContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := work.cancelUnstarted(cancelContext); err != nil {
			result = errors.Join(result, err)
		}
	}()
	if _, err := guard.SubmitSuspendedGCJob(ctx, database, client.BatchV1().Jobs(namespace), request, template); err != nil {
		return err
	}
	return work.run(ctx)
}

func scheduledGCResult(progress guard.GCJobProgress) error {
	if progress.State == "finished" && progress.Outcome == "succeeded" {
		return nil
	}
	return errScheduledGCFailed
}

func scheduledGCRequest(binding guard.StorageRegistration, slot string, keepCount uint) guard.CoordinationRequest {
	identity := sha256.Sum256([]byte("scheduled-registry-gc-v1\x00" + slot + "\x00" + binding.StorageID + "\x00" + binding.Generation))
	fingerprint := sha256.Sum256([]byte("scheduled-registry-gc-request-v1\x00" + hex.EncodeToString(identity[:]) + "\x00" + strconv.FormatUint(uint64(keepCount), 10)))
	return guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: hex.EncodeToString(identity[:]), Owner: "scheduled-registry-gc", Kind: "gc", Scope: "*", Fingerprint: hex.EncodeToString(fingerprint[:])}
}

func (h *CleanupCoordinationHandler) scheduledGCTrigger(ctx context.Context, triggerID string) (string, error) {
	if triggerID != "startup" {
		if !strings.HasPrefix(triggerID, "daily:") || len(triggerID) > 128 {
			return "", guard.ErrCoordinationChanged
		}
		if _, err := time.Parse(time.RFC3339, strings.TrimPrefix(triggerID, "daily:")); err != nil {
			return "", guard.ErrCoordinationChanged
		}
		return triggerID, nil
	}
	client, namespace, _, err := h.gcTarget()
	if err != nil || client == nil || namespace == "" {
		return "", guard.ErrCoordinationUnavailable
	}
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "name=rbd-chaos", Limit: 65})
	if err != nil || pods.ResourceVersion == "" || pods.Continue != "" || len(pods.Items) == 0 || len(pods.Items) > 64 {
		return "", guard.ErrCoordinationChanged
	}
	ownerName, ownerUID := "", ""
	for _, pod := range pods.Items {
		if pod.UID == "" || pod.DeletionTimestamp != nil || len(pod.OwnerReferences) != 1 {
			return "", guard.ErrCoordinationChanged
		}
		owner := pod.OwnerReferences[0]
		if owner.APIVersion != "apps/v1" || owner.Kind != "DaemonSet" || owner.Name == "" || owner.UID == "" || owner.Controller == nil || !*owner.Controller {
			return "", guard.ErrCoordinationChanged
		}
		if ownerUID != "" && (ownerUID != string(owner.UID) || ownerName != owner.Name) {
			return "", guard.ErrCoordinationChanged
		}
		ownerUID, ownerName = string(owner.UID), owner.Name
	}
	daemonSet, err := client.AppsV1().DaemonSets(namespace).Get(ctx, ownerName, metav1.GetOptions{})
	if err != nil || string(daemonSet.UID) != ownerUID || daemonSet.Generation <= 0 || daemonSet.ResourceVersion == "" || daemonSet.DeletionTimestamp != nil {
		return "", guard.ErrCoordinationChanged
	}
	return "startup:" + ownerUID + ":" + strconv.FormatInt(daemonSet.Generation, 10), nil
}

// scheduledRegistryBinding locates existing enrollment only. It cannot enroll a
// different directory or turn collecting storage into deletion-ready storage.
func (h *CleanupCoordinationHandler) scheduledRegistryBinding(ctx context.Context, client kubernetes.Interface, namespace, serviceName string) (guard.StorageRegistration, error) {
	service, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil || service.UID == "" || service.ResourceVersion == "" || len(service.Spec.Selector) == 0 {
		return guard.StorageRegistration{}, guard.ErrCoordinationUnavailable
	}
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(service.Spec.Selector).String(), Limit: 33})
	if err != nil || pods.ResourceVersion == "" || pods.Continue != "" || len(pods.Items) == 0 || len(pods.Items) > 32 {
		return guard.StorageRegistration{}, guard.ErrCoordinationUnavailable
	}
	volume, root := "", ""
	for _, pod := range pods.Items {
		observed, err := h.inspectRegistry(ctx, pod.Name, string(pod.UID))
		if err != nil || observed.Mount.VolumeUID == "" || observed.Root == "" || volume != "" && (volume != observed.Mount.VolumeUID || root != observed.Root) {
			return guard.StorageRegistration{}, guard.ErrCoordinationChanged
		}
		volume, root = observed.Mount.VolumeUID, observed.Root
	}
	stores, err := guard.DiscoverStores(h.database())
	if err != nil {
		return guard.StorageRegistration{}, err
	}
	var selected guard.StorageRegistration
	for _, store := range stores {
		binding, err := guard.StorageBinding(h.database(), store.StorageID, store.Generation)
		if err != nil {
			return guard.StorageRegistration{}, err
		}
		if binding.VolumeUID == volume && binding.RootPath == root && guard.IsRegistryStorageBinding(binding) {
			if selected.StorageID != "" {
				return guard.StorageRegistration{}, guard.ErrCoordinationChanged
			}
			selected = binding
		}
	}
	if selected.StorageID == "" {
		return guard.StorageRegistration{}, guard.ErrCoordinationUnavailable
	}
	return selected, nil
}

type scheduledGCWork struct {
	database           *gorm.DB
	client             kubernetes.Interface
	namespace, service string
	binding            guard.StorageRegistration
	request            guard.CoordinationRequest
}

func (w scheduledGCWork) currentTemplate(ctx context.Context) (*batchv1.Job, error) {
	return kubeidentity.BuildRegistryGCJob(ctx, w.client, w.namespace, w.service, w.binding, w.request)
}

func (w scheduledGCWork) verifySource(ctx context.Context, job *batchv1.Job) error {
	current, err := w.currentTemplate(ctx)
	if err != nil {
		return err
	}
	if !kubeidentity.SameRegistryGCSource(job, current) {
		return guard.ErrCoordinationChanged
	}
	return nil
}

func (w scheduledGCWork) run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		progress, err := guard.ReadGCJobProgress(w.database, w.request)
		if err != nil {
			return err
		}
		if progress.State == "finished" {
			return scheduledGCResult(progress)
		}
		if progress.State == "uncertain" {
			return guard.ErrCoordinationUncertain
		}
		job, err := guard.ReconcileGCJob(ctx, w.database, w.client.BatchV1().Jobs(w.namespace), w.request)
		if err != nil {
			return err
		}
		// The helper may have committed its outcome between the first DB read
		// and the Kubernetes observation. Read after seeing native termination so
		// an older exclusive receipt cannot misclassify a completed helper.
		progress, err = guard.ReadGCJobProgress(w.database, w.request)
		if err != nil {
			return err
		}
		if progress.State == "finished" {
			return scheduledGCResult(progress)
		}
		if progress.State == "uncertain" {
			return guard.ErrCoordinationUncertain
		}
		if gcExecutorState(job) == "unknown" {
			return guard.ErrCoordinationUncertain
		}
		switch progress.State {
		case "draining":
			if gcExecutorState(job) == "failed" {
				if err := w.cancelUnstarted(ctx); err != nil {
					return err
				}
				return errScheduledGCFailed
			}
			if err := w.verifySource(ctx, job); err != nil {
				return err
			}
			if _, err := guard.StartGCJob(ctx, w.database, w.client.BatchV1().Jobs(w.namespace), w.request); err != nil && !errors.Is(err, guard.ErrCoordinationBusy) {
				return err
			}
		case "restore_pending", "restoring":
			if err := w.restore(ctx); err == nil {
				return scheduledGCResult(guard.GCJobProgress{State: "finished", Outcome: progress.Outcome})
			} else if !errors.Is(err, guard.ErrCoordinationBusy) {
				return err
			}
		case "exclusive":
			if state := gcExecutorState(job); state == "failed" || state == "succeeded" {
				// A terminated helper without a durable outcome is not completion.
				return guard.ErrCoordinationUncertain
			}
		default:
			return guard.ErrCoordinationChanged
		}
		if err := waitScheduledGC(ctx); err != nil {
			return err
		}
	}
}

func waitScheduledGC(ctx context.Context) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (w scheduledGCWork) restore(ctx context.Context) error {
	binding, err := guard.ReadGCJobBinding(w.database, w.request)
	if err != nil || binding.Namespace != w.namespace {
		return guard.ErrCoordinationChanged
	}
	verify := func() error {
		job, err := guard.ReconcileGCJob(ctx, w.database, w.client.BatchV1().Jobs(w.namespace), w.request)
		if err != nil {
			return err
		}
		if err := w.verifySource(ctx, job); err != nil {
			return err
		}
		if _, err := kubeidentity.InspectTerminatedGCExecutor(ctx, w.client, w.service, job, binding.PodName, binding.PodUID, w.binding); err != nil {
			if errors.Is(err, kubeidentity.ErrExecutorRunning) {
				return guard.ErrCoordinationBusy
			}
			return guard.ErrCoordinationChanged
		}
		return nil
	}
	if err := verify(); err != nil {
		return err
	}
	if err := guard.BeginGCJobRestore(w.database, w.request, binding.JobUID, binding.PodUID); err != nil {
		return err
	}
	if err := verify(); err != nil {
		return err
	}
	return guard.FinishGCJobRestore(w.database, w.request, binding.JobUID, binding.PodUID)
}

func (w scheduledGCWork) cancelUnstarted(ctx context.Context) error {
	progress, err := guard.ReadGCJobProgress(w.database, w.request)
	if err != nil {
		return err
	}
	if progress.State == "finished" {
		return nil
	}
	if progress.State != "draining" {
		return guard.ErrCoordinationUncertain
	}
	binding, err := guard.ReadGCJobBinding(w.database, w.request)
	if errors.Is(err, guard.ErrGCJobNotPrepared) {
		return guard.CancelMaintenanceDrain(w.database, w.request)
	}
	if err != nil || binding.Namespace != w.namespace {
		return guard.ErrCoordinationChanged
	}
	job, err := guard.ReconcileGCJob(ctx, w.database, w.client.BatchV1().Jobs(w.namespace), w.request)
	if err != nil {
		return err
	}
	if err := w.verifySource(ctx, job); err != nil {
		return err
	}
	if gcExecutorState(job) != "failed" {
		if job.Spec.Suspend == nil || !*job.Spec.Suspend || job.Status.Active != 0 {
			return guard.ErrCoordinationUncertain
		}
		// Do not infer idle execution from Job.status alone: an original Pod may
		// already exist before its owner status has caught up.
		pods, err := w.client.CoreV1().Pods(w.namespace).List(ctx, metav1.ListOptions{Limit: 1025})
		if err != nil || pods.ResourceVersion == "" || pods.Continue != "" || len(pods.Items) > 1024 {
			return guard.ErrCoordinationUncertain
		}
		for _, pod := range pods.Items {
			for _, owner := range pod.OwnerReferences {
				if owner.UID == job.UID {
					return guard.ErrCoordinationUncertain
				}
			}
		}
	}
	// The storage-row transaction rejects any concurrent execution grant. A
	// later-started helper therefore cannot enter GC after this cancellation.
	if err := guard.CancelMaintenanceDrain(w.database, w.request); err != nil {
		return fmt.Errorf("scheduled GC drain remains protected: %w", err)
	}
	return nil
}
