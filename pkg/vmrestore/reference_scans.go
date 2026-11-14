package vmrestore

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (r *Resolver) snapshotReferences(ctx context.Context, scope Scope, resolved ResolvedVolume, ownNames map[string]bool, activeHandle string, summary *ReferenceSummary) error {
	contents, err := r.objects(ctx, vmSnapshotContentResource, scope.Namespace, true)
	if err != nil {
		return err
	}
	for _, content := range contents {
		sourceService := textAt(content.Object, "spec", "source", "virtualMachine", "metadata", "labels", "service_id")
		logical := false
		if sourceService == scope.ServiceID {
			for _, value := range sliceAt(content.Object, "spec", "source", "virtualMachine", "spec", "template", "spec", "volumes") {
				if asMap(value)["name"] == resolved.CanonicalName {
					logical = true
				}
			}
		}
		matched := false
		for _, value := range sliceAt(content.Object, "spec", "volumeBackups") {
			backup := asMap(value)
			name := textAt(backup, "persistentVolumeClaim", "metadata", "name")
			namespace := textAt(backup, "persistentVolumeClaim", "metadata", "namespace")
			uid := textAt(backup, "persistentVolumeClaim", "metadata", "uid")
			if namespace == "" {
				namespace = content.GetNamespace()
			}
			ref, physical := physicalPVC(resolved, namespace, name, uid)
			if logical && backup["volumeName"] != resolved.CanonicalName {
				continue
			}
			if !logical && !physical {
				continue
			}
			if uid == "" || !validUID(string(content.GetUID())) {
				return ErrReferenceUnavailable
			}
			if ref == nil {
				ref = &ObjectRef{Kind: "PersistentVolumeClaim", Namespace: namespace, Name: name, UID: uid}
			}
			summary.SnapshotRefs = pushReference(summary.SnapshotRefs, VolumeReference{Object: refOf("VirtualMachineSnapshotContent", &content), Reason: "vm_snapshot", Logical: logical, Physical: physical, PVC: ref})
			matched = true
		}
		// Read-only layout/installer metadata can be logically retained even without
		// a writable CSI backup. It must not be silently removed from Region metadata.
		if logical && !matched {
			summary.SnapshotRefs = pushReference(summary.SnapshotRefs, VolumeReference{Object: refOf("VirtualMachineSnapshotContent", &content), Reason: "vm_snapshot_layout", Logical: true})
		}
	}
	snapshots, err := r.objects(ctx, vmSnapshotResource, scope.Namespace, true)
	if err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		source := textAt(snapshot.Object, "spec", "source", "name")
		phase := textAt(snapshot.Object, "status", "phase")
		if ownNames[source] && (phase != "Succeeded" && phase != "Failed") {
			summary.InflightRefs = pushReference(summary.InflightRefs, VolumeReference{Object: refOf("VirtualMachineSnapshot", &snapshot), Reason: "vm_snapshot_pending", Logical: true, Physical: true, PVC: resolved.ActivePVC})
		}
	}
	csiContents, err := r.objects(ctx, csiSnapshotContentResource, "", true)
	if err != nil {
		return err
	}
	physicalContent := map[string]bool{}
	for _, content := range csiContents {
		handle := textAt(content.Object, "spec", "source", "volumeHandle")
		driver := textAt(content.Object, "spec", "driver")
		if activeHandle != "" && driver+"\x00"+handle == activeHandle {
			physicalContent[content.GetName()] = true
			summary.SnapshotRefs = pushReference(summary.SnapshotRefs, VolumeReference{Object: refOf("VolumeSnapshotContent", &content), Reason: "csi_snapshot", Physical: true, PVC: resolved.ActivePVC})
		}
	}
	snapshotsCSI, err := r.objects(ctx, csiSnapshotResource, scope.Namespace, true)
	if err != nil {
		return err
	}
	for _, snapshot := range snapshotsCSI {
		name := textAt(snapshot.Object, "spec", "source", "persistentVolumeClaimName")
		if resolved.ActivePVC == nil || name != resolved.ActivePVC.Name {
			continue
		}
		bound := textAt(snapshot.Object, "status", "boundVolumeSnapshotContentName")
		if bound == "" || physicalContent[bound] {
			summary.SnapshotRefs = pushReference(summary.SnapshotRefs, VolumeReference{Object: refOf("VolumeSnapshot", &snapshot), Reason: "csi_snapshot", Physical: true, PVC: resolved.ActivePVC})
		}
	}
	return nil
}

func sourcePVCMatches(obj map[string]interface{}, objectNamespace string, resolved ResolvedVolume) bool {
	if resolved.ActivePVC == nil {
		return false
	}
	for _, state := range []string{"spec", "status"} {
		name := textAt(obj, state, "source", "pvc", "name")
		namespace := textAt(obj, state, "source", "pvc", "namespace")
		if namespace == "" {
			namespace = objectNamespace
		}
		if name == resolved.ActivePVC.Name && namespace == resolved.ActivePVC.Namespace {
			return true
		}
	}
	return false
}
func (r *Resolver) cloneReferences(ctx context.Context, scope Scope, resolved ResolvedVolume, summary *ReferenceSummary) error {
	volumes, err := r.objects(ctx, dataVolumeResource, "", false)
	if err != nil {
		return err
	}
	needsDataSource := false
	for _, volume := range volumes {
		if textAt(volume.Object, "spec", "sourceRef", "kind") == "DataSource" {
			needsDataSource = true
		}
	}
	sources, err := r.objects(ctx, dataSourceResource, "", !needsDataSource)
	if err != nil {
		return err
	}
	sourceObjects := map[string]unstructured.Unstructured{}
	for _, source := range sources {
		sourceObjects[source.GetNamespace()+"/"+source.GetName()] = source
		if sourcePVCMatches(source.Object, source.GetNamespace(), resolved) {
			summary.ExternalRefs = pushReference(summary.ExternalRefs, VolumeReference{Object: refOf("DataSource", &source), Reason: "data_source", Physical: true, PVC: resolved.ActivePVC})
		}
	}
	for _, volume := range volumes {
		if resolved.ActiveDV != nil && volume.GetNamespace() == resolved.ActiveDV.Namespace && volume.GetName() == resolved.ActiveDV.Name && string(volume.GetUID()) == resolved.ActiveDV.UID {
			continue
		}
		matched := sourcePVCMatches(volume.Object, volume.GetNamespace(), resolved)
		sourceKind := textAt(volume.Object, "spec", "sourceRef", "kind")
		if sourceKind == "DataSource" {
			namespace := textAt(volume.Object, "spec", "sourceRef", "namespace")
			if namespace == "" {
				namespace = volume.GetNamespace()
			}
			name := textAt(volume.Object, "spec", "sourceRef", "name")
			if source, exists := sourceObjects[namespace+"/"+name]; exists && sourcePVCMatches(source.Object, source.GetNamespace(), resolved) {
				matched = true
			}
		}
		// Keep the existing lifecycle policy: declarations still using a clone source
		// block reclaim, even if a controller currently reports Succeeded.
		if matched {
			summary.ExternalRefs = pushReference(summary.ExternalRefs, VolumeReference{Object: refOf("DataVolume", &volume), Reason: "clone_source", Physical: true, PVC: resolved.ActivePVC})
		}
	}
	claims, err := r.core.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	for _, claim := range claims.Items {
		if resolved.ActivePVC == nil || sameObject(*resolved.ActivePVC, refOf("PersistentVolumeClaim", &claim)) {
			continue
		}
		matched := false
		if source := claim.Spec.DataSource; source != nil && source.Kind == "PersistentVolumeClaim" && source.Name == resolved.ActivePVC.Name && claim.Namespace == resolved.ActivePVC.Namespace {
			matched = true
		}
		if source := claim.Spec.DataSourceRef; source != nil && source.Kind == "PersistentVolumeClaim" {
			namespace := claim.Namespace
			if source.Namespace != nil && *source.Namespace != "" {
				namespace = *source.Namespace
			}
			if source.Name == resolved.ActivePVC.Name && namespace == resolved.ActivePVC.Namespace {
				matched = true
			}
		}
		if matched {
			summary.ExternalRefs = pushReference(summary.ExternalRefs, VolumeReference{Object: refOf("PersistentVolumeClaim", &claim), Reason: "pvc_clone_source", Physical: true, PVC: resolved.ActivePVC})
		}
	}
	return nil
}
