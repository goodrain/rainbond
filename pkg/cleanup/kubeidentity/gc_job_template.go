package kubeidentity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// BuildRegistryGCJob derives the executor from the configured Registry Service.
// The installed coordinator image must include the native GC helper and be
// pinned by the installer. No secret contents, caller YAML or caller paths are
// accepted. Construction alone does not register or start an operation.
func BuildRegistryGCJob(ctx context.Context, client kubernetes.Interface, namespace, serviceName string, binding coordination.StorageRegistration, r coordination.CoordinationRequest) (*batchv1.Job, error) {
	if client == nil || r.Kind != "gc" || r.StorageID != binding.StorageID || r.Generation != binding.Generation {
		return nil, ErrBinding
	}
	if _, err := binding.Fingerprint(); err != nil {
		return nil, ErrBinding
	}
	service, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil || service.UID == "" || service.ResourceVersion == "" || len(service.Spec.Selector) == 0 {
		return nil, ErrBinding
	}
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(service.Spec.Selector).String(), Limit: 33})
	if err != nil || pods.ResourceVersion == "" || pods.Continue != "" || len(pods.Items) == 0 || len(pods.Items) > 32 {
		return nil, ErrBinding
	}
	sort.Slice(pods.Items, func(i, j int) bool { return pods.Items[i].Name < pods.Items[j].Name })
	cleaners, err := inspectLegacyCleaners(ctx, client, namespace)
	if err != nil {
		return nil, err
	}
	var result *batchv1.Job
	source := struct {
		Cleaners   []legacyCleanerObservation
		ServiceUID string
		Service    corev1.ServiceSpec
		Pods       []struct {
			UID         string
			Spec        corev1.PodSpec
			NativeImage string
		}
	}{Cleaners: cleaners, ServiceUID: string(service.UID), Service: service.Spec}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Annotations["rainbond.io/registry-gc-executor"] != "v1" || pod.Spec.NodeName == "" {
			return nil, ErrBinding
		}
		prepared, err := InspectNativeRegistry(ctx, client, namespace, serviceName, pod.Name, string(pod.UID))
		if err != nil || prepared.Mount.PodVersion != pod.ResourceVersion || prepared.Root != binding.RootPath || prepared.Mount.VolumeUID != binding.VolumeUID {
			return nil, ErrBinding
		}
		native, err := containerByName(pod, prepared.Container)
		if err != nil {
			return nil, err
		}
		purging, err := literalEnv(native, "REGISTRY_STORAGE_MAINTENANCE_UPLOADPURGING_ENABLED")
		if err != nil || purging != "false" {
			return nil, ErrBinding
		}
		var sidecar *corev1.Container
		for n := range pod.Spec.Containers {
			candidate := &pod.Spec.Containers[n]
			if len(candidate.Command) == 1 && candidate.Command[0] == "/registry-coordinator" {
				if sidecar != nil {
					return nil, ErrBinding
				}
				sidecar = candidate
			}
		}
		if sidecar == nil {
			return nil, ErrBinding
		}
		if _, err := InspectRegistryIngress(service, pod, native.Name, sidecar.Name); err != nil {
			return nil, err
		}
		args, err := explicitArguments(sidecar)
		if err != nil || args["storage-id"] != binding.StorageID || args["storage-generation"] != binding.Generation || args["volume-uid"] != binding.VolumeUID || args["registry-path"] != binding.RootPath {
			return nil, ErrBinding
		}
		request := RegistryMountRequest{Namespace: namespace, Service: serviceName, Pod: pod.Name, PodUID: string(pod.UID), RegistryContainer: native.Name, CoordinatorContainer: sidecar.Name, RegistryRoot: binding.RootPath, CoordinatorRoot: args["storage-root"]}
		observed, err := InspectRegistryMount(ctx, client, request)
		if err != nil || observed.VolumeUID != binding.VolumeUID || observed.PodVersion != pod.ResourceVersion {
			return nil, ErrBinding
		}
		if !pinnedRuntimeImage(pod, sidecar) {
			return nil, ErrBinding
		}
		job, err := registryGCJob(pod, native, sidecar, args, binding, r)
		if err != nil {
			return nil, err
		}
		nativeImage := ""
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == native.Name {
				if nativeImage != "" || status.State.Running == nil || status.ImageID == "" {
					return nil, ErrBinding
				}
				nativeImage = status.ImageID
			}
		}
		if nativeImage == "" {
			return nil, ErrBinding
		}
		source.Pods = append(source.Pods, struct {
			UID         string
			Spec        corev1.PodSpec
			NativeImage string
		}{string(pod.UID), pod.Spec, nativeImage})
		if result == nil {
			result = job
		} else if result.Spec.Template.Spec.Containers[0].Image != job.Spec.Template.Spec.Containers[0].Image {
			return nil, ErrBinding
		}
	}
	current, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil || current.UID != service.UID || current.ResourceVersion != service.ResourceVersion {
		return nil, ErrBinding
	}
	raw, err := json.Marshal(source)
	if err != nil {
		return nil, ErrBinding
	}
	sum := sha256.Sum256(raw)
	result.Spec.Template.Annotations = map[string]string{"rainbond.io/gc-source-fingerprint": hex.EncodeToString(sum[:])}
	return result, nil
}

func pinnedRuntimeImage(pod *corev1.Pod, c *corev1.Container) bool {
	parts := strings.Split(c.Image, "@sha256:")
	if len(parts) != 2 || parts[0] == "" || len(parts[1]) != 64 {
		return false
	}
	raw, err := hex.DecodeString(parts[1])
	if err != nil || hex.EncodeToString(raw) != parts[1] {
		return false
	}
	matches := 0
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name != c.Name {
			continue
		}
		matches++
		image := strings.TrimPrefix(strings.TrimPrefix(status.ImageID, "docker-pullable://"), "containerd://")
		if status.State.Running == nil || status.ContainerID == "" || (image != c.Image && image != "sha256:"+parts[1]) {
			return false
		}
	}
	return matches == 1
}

func registryGCJob(pod *corev1.Pod, native, sidecar *corev1.Container, args map[string]string, binding coordination.StorageRegistration, r coordination.CoordinationRequest) (*batchv1.Job, error) {
	endpoint, err := url.Parse(args["coordination-api"])
	allowHTTP := args["allow-internal-http"] == "true"
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") || (endpoint.Scheme != "https" && !(endpoint.Scheme == "http" && allowHTTP)) {
		return nil, ErrBinding
	}
	mount, relative, err := effectiveMount(native, binding.RootPath)
	if err != nil || mount.ReadOnly || mount.MountPropagation != nil && *mount.MountPropagation != corev1.MountPropagationNone {
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
	// Preserve the exact native mount/subpath and node; never mount another PVC by
	// a browser-provided name. The executor rechecks the current physical UID.
	if below(binding.RootPath, "/registry-gc") || below(binding.RootPath, "/bin/registry") || below("/tmp", binding.RootPath) || below(binding.RootPath, "/tmp") {
		return nil, ErrBinding
	}
	mount.MountPath = binding.RootPath
	mount.SubPath = relative
	data.Name = "gc-data"
	mount.Name = data.Name
	no, yes := false, true
	zero, one := int32(0), int32(1)
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: pod.Namespace}, Spec: batchv1.JobSpec{Suspend: &yes, BackoffLimit: &zero, Parallelism: &one, Completions: &one, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"rainbond.io/task": "registry-gc"}}, Spec: corev1.PodSpec{NodeName: pod.Spec.NodeName, RestartPolicy: corev1.RestartPolicyNever, AutomountServiceAccountToken: &no, ServiceAccountName: pod.Spec.ServiceAccountName, ImagePullSecrets: append([]corev1.LocalObjectReference(nil), pod.Spec.ImagePullSecrets...), Volumes: []corev1.Volume{*data, {Name: "gc-tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}}}}}
	raw, err := json.Marshal(struct {
		Binding coordination.StorageRegistration `json:"binding"`
		Request coordination.CoordinationRequest `json:"request"`
	}{binding, r})
	if err != nil {
		return nil, ErrBinding
	}
	c := corev1.Container{Name: "gc", Image: sidecar.Image, Command: []string{"/registry-gc"}, Args: []string{"--coordination-api=" + endpoint.String(), "--allow-internal-http=" + strconv.FormatBool(allowHTTP)}, VolumeMounts: []corev1.VolumeMount{mount, {Name: "gc-tmp", MountPath: "/tmp"}}, Resources: *sidecar.Resources.DeepCopy(), Env: []corev1.EnvVar{{Name: "CLEANUP_GC_OPERATION", Value: string(raw)}, {Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.name"}}}, {Name: "POD_UID", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.uid"}}}}, SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: &no, ReadOnlyRootFilesystem: &yes, Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}}}
	if native.SecurityContext != nil {
		c.SecurityContext.RunAsUser = native.SecurityContext.RunAsUser
		c.SecurityContext.RunAsGroup = native.SecurityContext.RunAsGroup
		c.SecurityContext.RunAsNonRoot = native.SecurityContext.RunAsNonRoot
	}
	if pod.Spec.SecurityContext != nil {
		security := pod.Spec.SecurityContext.DeepCopy()
		if security.FSGroup != nil {
			security.SupplementalGroups = append(security.SupplementalGroups, *security.FSGroup)
		}
		security.FSGroup = nil
		security.FSGroupChangePolicy = nil
		job.Spec.Template.Spec.SecurityContext = security
	}
	if value := args["coordination-server-name"]; value != "" {
		c.Args = append(c.Args, "--coordination-server-name="+value)
	}
	for _, key := range []string{"credential-file", "coordination-ca-file", "coordination-client-cert-file", "coordination-client-key-file"} {
		file := args[key]
		if file == "" {
			if key == "credential-file" {
				return nil, ErrBinding
			}
			continue
		}
		if !cleanRoot(file) || below(binding.RootPath, file) || below("/tmp", file) {
			return nil, ErrBinding
		}
		credentialMount, _, err := effectiveMount(sidecar, file)
		if err != nil || !credentialMount.ReadOnly || credentialMount.Name == "gc-data" || credentialMount.Name == "gc-tmp" || below(credentialMount.MountPath, binding.RootPath) || below(binding.RootPath, credentialMount.MountPath) || below(credentialMount.MountPath, "/bin/registry") || below(credentialMount.MountPath, "/registry-gc") {
			return nil, ErrBinding
		}
		var source *corev1.Volume
		for i := range pod.Spec.Volumes {
			if pod.Spec.Volumes[i].Name == credentialMount.Name {
				source = pod.Spec.Volumes[i].DeepCopy()
			}
		}
		if source == nil {
			return nil, ErrBinding
		}
		// Copy references only; never GET a Secret or place its value in a Job.
		if source.Secret == nil && source.ConfigMap == nil {
			return nil, ErrBinding
		}
		exists := false
		for _, v := range job.Spec.Template.Spec.Volumes {
			if v.Name == source.Name {
				if !reflect.DeepEqual(v, *source) {
					return nil, ErrBinding
				}
				exists = true
			}
		}
		if !exists {
			job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, *source)
		}
		exists = false
		for _, m := range c.VolumeMounts {
			if m.Name == credentialMount.Name && m.MountPath == credentialMount.MountPath {
				if !reflect.DeepEqual(m, credentialMount) {
					return nil, ErrBinding
				}
				exists = true
			}
		}
		if !exists {
			c.VolumeMounts = append(c.VolumeMounts, credentialMount)
		}
		c.Args = append(c.Args, "--"+key+"="+file)
	}
	if (args["coordination-client-cert-file"] == "") != (args["coordination-client-key-file"] == "") {
		return nil, ErrBinding
	}
	job.Spec.Template.Spec.Containers = []corev1.Container{c}
	return job, nil
}

// SameRegistryGCSource compares the installer-derived source snapshot retained
// inside the durably hashed Job template with a fresh Kubernetes observation.
func SameRegistryGCSource(original, current *batchv1.Job) bool {
	if original == nil || current == nil || len(original.Spec.Template.Spec.Containers) != 1 || len(current.Spec.Template.Spec.Containers) != 1 {
		return false
	}
	before := original.Spec.Template.Annotations["rainbond.io/gc-source-fingerprint"]
	after := current.Spec.Template.Annotations["rainbond.io/gc-source-fingerprint"]
	return len(before) == 64 && before == after && original.Spec.Template.Spec.Containers[0].Image == current.Spec.Template.Spec.Containers[0].Image
}
