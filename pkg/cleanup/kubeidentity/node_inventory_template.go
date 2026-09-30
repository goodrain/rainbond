package kubeidentity

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NodeInventorySettings are operator-owned installation settings, not scan input.
type NodeInventorySettings struct {
	Region, Image, ScanID string
	IncludePackages       bool
}

// BuildManagedCacheInventoryJob constructs a one-shot read-only cache collector.
// The caller must persist and revalidate the template before starting it. It does
// not create Kubernetes resources, change readiness, or grant native deletion.
func BuildManagedCacheInventoryJob(ctx context.Context, client kubernetes.Interface, namespace, sourcePod, sourceUID string, binding coordination.StorageRegistration, settings NodeInventorySettings) (*batchv1.Job, error) {
	if _, err := binding.Fingerprint(); err != nil || binding.RootPath != "/cache/build" || settings.Region == "" || len(settings.Region) > 63 {
		return nil, ErrBinding
	}
	sum := sha256.Sum256([]byte("managed-build-cache\x00" + binding.VolumeUID))
	parts := strings.Split(settings.Image, "@sha256:")
	if binding.StorageID != hex.EncodeToString(sum[:]) || len(parts) != 2 || parts[0] == "" || len(parts[1]) != 64 {
		return nil, ErrBinding
	}
	digest, err := hex.DecodeString(parts[1])
	if err != nil || hex.EncodeToString(digest) != parts[1] {
		return nil, ErrBinding
	}
	observed, err := InspectManagedBuildCacheSource(ctx, client, namespace, sourcePod, sourceUID)
	if err != nil || observed.Mount.VolumeUID != binding.VolumeUID {
		return nil, ErrBinding
	}
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, sourcePod, metav1.GetOptions{})
	if err != nil || string(pod.UID) != sourceUID || pod.ResourceVersion != observed.Mount.PodVersion {
		return nil, ErrBinding
	}
	mount, relative, err := effectiveMount(&pod.Spec.Containers[0], binding.RootPath)
	if err != nil {
		return nil, ErrBinding
	}
	var data *corev1.Volume
	for i := range pod.Spec.Volumes {
		if pod.Spec.Volumes[i].Name == mount.Name {
			data = pod.Spec.Volumes[i].DeepCopy()
		}
	}
	if data == nil {
		return nil, ErrBinding
	}
	data.Name = "node-cache"
	if data.PersistentVolumeClaim != nil {
		data.PersistentVolumeClaim.ReadOnly = true
	}
	mount.Name = data.Name
	mount.MountPath = "/cache/build"
	mount.SubPath = relative
	mount.ReadOnly = true
	volumes := []corev1.Volume{*data}
	mounts := []corev1.VolumeMount{mount}
	roots := []map[string]string{{"kind": "cache", "storageId": binding.StorageID, "path": "/cache/build"}}
	runtimeEndpoint := ""
	if runtimeVolume, runtimeMount, ok := runtimeInventoryMount(pod); ok {
		volumes = append(volumes, runtimeVolume)
		mounts = append(mounts, runtimeMount)
		runtimeEndpoint = "unix:///runtime/containerd.sock"
	}
	if settings.IncludePackages {
		packageVolume, packageMount, packageRoots, err := packageInventoryMount(ctx, client, pod)
		if err != nil {
			return nil, err
		}
		volumes = append(volumes, packageVolume)
		mounts = append(mounts, packageMount)
		roots = append(roots, packageRoots...)
	}
	raw, err := json.Marshal(map[string]interface{}{"scanId": settings.ScanID, "region": settings.Region, "node": observed.Mount.NodeName, "nodeUid": observed.NodeUID, "runtimeEndpoint": runtimeEndpoint, "roots": roots})
	if err != nil {
		return nil, ErrBinding
	}
	no, yes := false, true
	zero, one := int32(0), int32(1)
	root, readerGroup := int64(0), int64(10001)
	deadline := int64(180)
	c := corev1.Container{Name: "node-inventory", Image: settings.Image, Command: []string{"/app/node-inventory"}, Args: []string{"--output=-"}, Env: []corev1.EnvVar{{Name: "CLEANUP_NODE_INVENTORY_BASE64", Value: base64.StdEncoding.EncodeToString(raw)}}, VolumeMounts: mounts, SecurityContext: &corev1.SecurityContext{RunAsUser: &root, RunAsGroup: &readerGroup, AllowPrivilegeEscalation: &no, ReadOnlyRootFilesystem: &yes, Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}}}
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: namespace}, Spec: batchv1.JobSpec{Suspend: &yes, BackoffLimit: &zero, Parallelism: &one, Completions: &one, ActiveDeadlineSeconds: &deadline, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"rainbond.io/node-source-pod": sourcePod, "rainbond.io/node-source-uid": sourceUID}}, Spec: corev1.PodSpec{NodeName: observed.Mount.NodeName, RestartPolicy: corev1.RestartPolicyNever, AutomountServiceAccountToken: &no, ImagePullSecrets: append([]corev1.LocalObjectReference(nil), pod.Spec.ImagePullSecrets...), Containers: []corev1.Container{c}, Volumes: volumes}}}}, nil
}

// runtimeInventoryMount copies only Rainbond's exact containerd socket mount
// into the read-only inventory Job. Missing or changed mounts simply leave CRI
// coverage unconfigured; filesystem inventory remains independently useful.
func runtimeInventoryMount(pod *corev1.Pod) (corev1.Volume, corev1.VolumeMount, bool) {
	if pod == nil || len(pod.Spec.Containers) != 1 {
		return corev1.Volume{}, corev1.VolumeMount{}, false
	}
	const source = "/run/containerd/containerd.sock"
	var observed *corev1.VolumeMount
	for index := range pod.Spec.Containers[0].VolumeMounts {
		mount := &pod.Spec.Containers[0].VolumeMounts[index]
		if mount.MountPath == source {
			if observed != nil {
				return corev1.Volume{}, corev1.VolumeMount{}, false
			}
			observed = mount
		}
	}
	if observed == nil || observed.SubPath != "" || observed.SubPathExpr != "" || (observed.MountPropagation != nil && *observed.MountPropagation != corev1.MountPropagationNone) {
		return corev1.Volume{}, corev1.VolumeMount{}, false
	}
	var volume *corev1.Volume
	for index := range pod.Spec.Volumes {
		if pod.Spec.Volumes[index].Name == observed.Name {
			if volume != nil {
				return corev1.Volume{}, corev1.VolumeMount{}, false
			}
			volume = &pod.Spec.Volumes[index]
		}
	}
	if volume == nil || volume.HostPath == nil || volume.HostPath.Path != source || (volume.HostPath.Type != nil && *volume.HostPath.Type != corev1.HostPathSocket) {
		return corev1.Volume{}, corev1.VolumeMount{}, false
	}
	copy := *volume.DeepCopy()
	copy.Name = "node-runtime"
	mount := corev1.VolumeMount{Name: copy.Name, MountPath: "/runtime/containerd.sock", ReadOnly: true}
	return copy, mount, true
}
