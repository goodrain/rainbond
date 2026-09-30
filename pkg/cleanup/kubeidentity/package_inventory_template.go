package kubeidentity

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func packageStorageID(volumeUID, kind string) string {
	sum := sha256.Sum256([]byte("node-upload-packages\x00" + volumeUID + "\x00" + kind))
	return hex.EncodeToString(sum[:])
}

func packageInventoryMount(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod) (corev1.Volume, corev1.VolumeMount, []map[string]string, error) {
	var denied corev1.Volume
	if client == nil || pod == nil || len(pod.Spec.Containers) != 1 {
		return denied, corev1.VolumeMount{}, nil, ErrBinding
	}
	mount, relative, err := effectiveMount(&pod.Spec.Containers[0], "/grdata/package_build")
	if err != nil || (mount.MountPropagation != nil && *mount.MountPropagation != corev1.MountPropagationNone) {
		return denied, mount, nil, ErrBinding
	}
	observed, err := inspectVolumeMode(ctx, client, pod, mount.Name, relative, true)
	if err != nil {
		return denied, mount, nil, ErrBinding
	}
	var data *corev1.Volume
	for _, v := range pod.Spec.Volumes {
		if v.Name == mount.Name {
			data = v.DeepCopy()
		}
	}
	if data == nil {
		return denied, mount, nil, ErrBinding
	}
	data.Name = "node-packages"
	if data.PersistentVolumeClaim != nil {
		data.PersistentVolumeClaim.ReadOnly = true
	}
	mount.Name, mount.MountPath, mount.SubPath, mount.ReadOnly = data.Name, "/packages", relative, true
	roots := []map[string]string{
		{"kind": "upload_events", "storageId": packageStorageID(observed.VolumeUID, "upload_events"), "path": "/packages/temp/events"},
		{"kind": "upload_components", "storageId": packageStorageID(observed.VolumeUID, "upload_components"), "path": "/packages/components"},
	}
	return *data, mount, roots, nil
}

// The collector descriptor binds package reports to the actual observed volume,
// independently of the cache volume and its deletion certification.
func verifyPackageInventoryMount(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod) error {
	if pod == nil || len(pod.Spec.Containers) != 1 {
		return ErrBinding
	}
	c := &pod.Spec.Containers[0]
	mount, relative, err := effectiveMount(c, "/packages")
	if err != nil || !mount.ReadOnly || mount.Name != "node-packages" {
		return ErrBinding
	}
	observed, err := inspectVolumeMode(ctx, client, pod, mount.Name, relative, true)
	if err != nil {
		return ErrBinding
	}
	encoded := ""
	for _, v := range c.Env {
		if v.Name == "CLEANUP_NODE_INVENTORY_BASE64" {
			if encoded != "" || v.ValueFrom != nil {
				return ErrBinding
			}
			encoded = v.Value
		}
	}
	if len(encoded) > base64.StdEncoding.EncodedLen(64<<10) {
		return ErrBinding
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ErrBinding
	}
	var config struct {
		Roots []struct {
			Kind      string `json:"kind"`
			StorageID string `json:"storageId"`
			Path      string `json:"path"`
		} `json:"roots"`
	}
	if json.Unmarshal(raw, &config) != nil {
		return ErrBinding
	}
	seen := map[string]bool{}
	for _, root := range config.Roots {
		if root.Kind != "upload_events" && root.Kind != "upload_components" {
			continue
		}
		path := "/packages/temp/events"
		if root.Kind == "upload_components" {
			path = "/packages/components"
		}
		if seen[root.Kind] || root.Path != path || root.StorageID != packageStorageID(observed.VolumeUID, root.Kind) {
			return ErrBinding
		}
		seen[root.Kind] = true
	}
	if len(seen) != 2 {
		return ErrBinding
	}
	return nil
}
