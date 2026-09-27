package kubeidentity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// ErrBinding rejects unverified or changed Kubernetes storage identity.
var ErrBinding = errors.New("registry Kubernetes storage binding unavailable or changed")

// RegistryMountRequest uses the system namespace/service chosen by the control
// plane, not arbitrary browser-supplied Pod selectors or storage paths.
type RegistryMountRequest struct {
	Namespace, Service, Pod, PodUID         string
	RegistryContainer, CoordinatorContainer string
	RegistryRoot, CoordinatorRoot           string
}

// RegistryMountObservation binds the shared directory to current Kubernetes UIDs.
// This proves a mount relationship, not reference completeness or write readiness.
type RegistryMountObservation struct {
	PodUID       string
	PodVersion   string
	VolumeUID    string
	RelativeRoot string
	NodeName     string
}

func cleanRoot(value string) bool {
	return path.IsAbs(value) && value != "/" && path.Clean(value) == value && !strings.ContainsAny(value, "\x00\\")
}
func subpath(value string) bool {
	if value == "" {
		return true
	}
	if path.IsAbs(value) || path.Clean(value) != value || value == "." || strings.ContainsAny(value, "\x00\\") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}
func below(root, candidate string) bool {
	return candidate == root || strings.HasPrefix(candidate, strings.TrimSuffix(root, "/")+"/")
}
func containerByName(pod *corev1.Pod, name string) (*corev1.Container, error) {
	var found *corev1.Container
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == name {
			if found != nil {
				return nil, ErrBinding
			}
			found = &pod.Spec.Containers[i]
		}
	}
	if found == nil {
		return nil, ErrBinding
	}
	return found, nil
}
func effectiveMount(container *corev1.Container, root string) (corev1.VolumeMount, string, error) {
	var selected *corev1.VolumeMount
	for i := range container.VolumeMounts {
		mount := &container.VolumeMounts[i]
		if mount.MountPath != "/" && !cleanRoot(mount.MountPath) {
			return corev1.VolumeMount{}, "", ErrBinding
		}
		if mount.MountPath != root && below(root, mount.MountPath) {
			return corev1.VolumeMount{}, "", ErrBinding
		}
		if below(mount.MountPath, root) {
			if selected != nil && len(mount.MountPath) == len(selected.MountPath) {
				return corev1.VolumeMount{}, "", ErrBinding
			}
			if selected == nil || len(mount.MountPath) > len(selected.MountPath) {
				selected = mount
			}
		}
	}
	if selected == nil || selected.SubPathExpr != "" || !subpath(selected.SubPath) {
		return corev1.VolumeMount{}, "", ErrBinding
	}
	relative := strings.TrimPrefix(strings.TrimPrefix(root, selected.MountPath), "/")
	result := path.Join(selected.SubPath, relative)
	if result == "." {
		result = ""
	}
	return *selected, result, nil
}
func volumeIdentity(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

// InspectRegistryMount queries actual objects and rejects replaced identities,
// mismatched subdirectories, unsupported volume types and overlay mounts.
func InspectRegistryMount(ctx context.Context, client kubernetes.Interface, r RegistryMountRequest) (RegistryMountObservation, error) {
	denied := RegistryMountObservation{}
	if client == nil || r.Namespace == "" || r.Service == "" || r.Pod == "" || r.PodUID == "" || r.RegistryContainer == r.CoordinatorContainer || !cleanRoot(r.RegistryRoot) || !cleanRoot(r.CoordinatorRoot) {
		return denied, ErrBinding
	}
	service, err := client.CoreV1().Services(r.Namespace).Get(ctx, r.Service, metav1.GetOptions{})
	if err != nil || len(service.Spec.Selector) == 0 || service.Spec.Type == corev1.ServiceTypeExternalName {
		return denied, ErrBinding
	}
	pod, err := client.CoreV1().Pods(r.Namespace).Get(ctx, r.Pod, metav1.GetOptions{})
	if err != nil || string(pod.UID) != r.PodUID || pod.DeletionTimestamp != nil || !labels.SelectorFromSet(service.Spec.Selector).Matches(labels.Set(pod.Labels)) {
		return denied, ErrBinding
	}
	native, err := containerByName(pod, r.RegistryContainer)
	if err != nil {
		return denied, err
	}
	sidecar, err := containerByName(pod, r.CoordinatorContainer)
	if err != nil {
		return denied, err
	}
	nativeMount, nativeRelative, err := effectiveMount(native, r.RegistryRoot)
	if err != nil {
		return denied, err
	}
	sidecarMount, sidecarRelative, err := effectiveMount(sidecar, r.CoordinatorRoot)
	if err != nil {
		return denied, err
	}
	if nativeMount.Name != sidecarMount.Name || nativeRelative != sidecarRelative || nativeMount.ReadOnly || !sidecarMount.ReadOnly {
		return denied, ErrBinding
	}
	return inspectVolume(ctx, client, pod, nativeMount.Name, nativeRelative)
}

func inspectVolume(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod, volumeName, relative string) (RegistryMountObservation, error) {
	return inspectVolumeMode(ctx, client, pod, volumeName, relative, false)
}

// allowReadOnly is for observational collectors only; native writers retain the
// original writable-volume requirement through inspectVolume.
func inspectVolumeMode(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod, volumeName, relative string, allowReadOnly bool) (RegistryMountObservation, error) {
	denied := RegistryMountObservation{}
	var volume *corev1.Volume
	for i := range pod.Spec.Volumes {
		if pod.Spec.Volumes[i].Name == volumeName {
			if volume != nil {
				return denied, ErrBinding
			}
			volume = &pod.Spec.Volumes[i]
		}
	}
	if volume == nil {
		return denied, ErrBinding
	}
	observed := RegistryMountObservation{PodUID: string(pod.UID), PodVersion: pod.ResourceVersion, RelativeRoot: relative, NodeName: pod.Spec.NodeName}
	if source := volume.PersistentVolumeClaim; source != nil {
		if source.ReadOnly && !allowReadOnly {
			return denied, ErrBinding
		}
		pvc, err := client.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(ctx, source.ClaimName, metav1.GetOptions{})
		if err != nil || pvc.UID == "" || pvc.DeletionTimestamp != nil || pvc.Status.Phase != corev1.ClaimBound || pvc.Spec.VolumeName == "" || (pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode != corev1.PersistentVolumeFilesystem) {
			return denied, ErrBinding
		}
		pv, err := client.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil || pv.UID == "" || pv.DeletionTimestamp != nil || pv.Status.Phase != corev1.VolumeBound || pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.UID != pvc.UID || pv.Spec.ClaimRef.Name != pvc.Name || pv.Spec.ClaimRef.Namespace != pod.Namespace || (pv.Spec.VolumeMode != nil && *pv.Spec.VolumeMode != corev1.PersistentVolumeFilesystem) {
			return denied, ErrBinding
		}
		observed.VolumeUID = volumeIdentity("pvc", pod.Namespace, string(pvc.UID), string(pv.UID), relative)
	} else if source := volume.HostPath; source != nil {
		if !path.IsAbs(source.Path) || path.Clean(source.Path) != source.Path || strings.ContainsAny(source.Path, "\x00\\") || pod.Spec.NodeName == "" {
			return denied, ErrBinding
		}
		if source.Type != nil && *source.Type != corev1.HostPathDirectory && *source.Type != corev1.HostPathDirectoryOrCreate && *source.Type != corev1.HostPathUnset {
			return denied, ErrBinding
		}
		node, err := client.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
		if err != nil || node.UID == "" || node.DeletionTimestamp != nil {
			return denied, ErrBinding
		}
		observed.VolumeUID = volumeIdentity("hostpath", string(node.UID), path.Join(source.Path, relative))
	} else {
		return denied, ErrBinding
	}
	return observed, nil
}
