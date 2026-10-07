package kubeidentity

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"time"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const inventoryReportLimit = 32 << 20

func matchesPinnedImageID(expected, observed string) bool {
	if strings.Count(expected, "@") != 1 {
		return false
	}
	_, digest, found := strings.Cut(expected, "@")
	encoded := strings.TrimPrefix(digest, "sha256:")
	decoded, err := hex.DecodeString(encoded)
	if !found || !strings.HasPrefix(digest, "sha256:") || len(encoded) != 64 || err != nil || hex.EncodeToString(decoded) != encoded {
		return false
	}
	observed = strings.TrimPrefix(strings.TrimPrefix(observed, "docker-pullable://"), "containerd://")
	if observed == expected || observed == digest {
		return true
	}
	if strings.Count(observed, "@") != 1 {
		return false
	}
	_, observedDigest, found := strings.Cut(observed, "@")
	return found && observedDigest == digest
}

// ReadNodeInventoryReport retrieves only the verified successful collector's
// bounded output. No cross-namespace volume or collector credentials are needed.
func ReadNodeInventoryReport(ctx context.Context, client kubernetes.Interface, job *batchv1.Job, binding coordination.StorageRegistration, settings NodeInventorySettings) (json.RawMessage, error) {
	return readNodeInventoryReport(ctx, client, job, binding, settings, func(ctx context.Context, namespace, pod, container string) (io.ReadCloser, error) {
		limit := int64(inventoryReportLimit + 1)
		return client.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{Container: container, LimitBytes: &limit}).Stream(ctx)
	})
}

func readNodeInventoryReport(ctx context.Context, client kubernetes.Interface, job *batchv1.Job, binding coordination.StorageRegistration, settings NodeInventorySettings, read func(context.Context, string, string, string) (io.ReadCloser, error)) (json.RawMessage, error) {
	if client == nil || job == nil || job.UID == "" || job.ResourceVersion == "" || job.DeletionTimestamp != nil || job.Spec.Suspend == nil || *job.Spec.Suspend || job.Status.Succeeded != 1 || job.Status.Failed != 0 || binding.RootPath != "/cache/build" {
		return nil, ErrBinding
	}
	pods, err := client.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "job-name=" + job.Name, Limit: 2})
	if err != nil || pods.ResourceVersion == "" || pods.Continue != "" || len(pods.Items) != 1 {
		return nil, ErrBinding
	}
	pod := &pods.Items[0]
	if pod.UID == "" || pod.ResourceVersion == "" || pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodSucceeded || len(pod.OwnerReferences) != 1 {
		return nil, ErrBinding
	}
	owner := pod.OwnerReferences[0]
	if owner.APIVersion != "batch/v1" || owner.Kind != "Job" || owner.Name != job.Name || owner.UID != job.UID || owner.Controller == nil || !*owner.Controller {
		return nil, ErrBinding
	}
	spec, expected := pod.Spec, job.Spec.Template.Spec
	if spec.NodeName == "" || spec.NodeName != expected.NodeName || spec.HostNetwork || spec.HostPID || spec.HostIPC || spec.RestartPolicy != corev1.RestartPolicyNever || spec.AutomountServiceAccountToken == nil || *spec.AutomountServiceAccountToken || len(spec.Containers) != 1 || len(spec.InitContainers) != 0 || len(spec.EphemeralContainers) != 0 || !reflect.DeepEqual(spec.Containers, expected.Containers) || !reflect.DeepEqual(spec.Volumes, expected.Volumes) || !reflect.DeepEqual(spec.SecurityContext, expected.SecurityContext) {
		return nil, ErrBinding
	}
	c := &spec.Containers[0]
	if len(c.Command) != 1 || c.Command[0] != "/app/node-inventory" || len(c.Args) != 1 || c.Args[0] != "--output=-" || !strings.Contains(c.Image, "@sha256:") || len(pod.Status.ContainerStatuses) != 1 {
		return nil, ErrBinding
	}
	status := pod.Status.ContainerStatuses[0]
	term := status.State.Terminated
	if status.Name != c.Name || status.RestartCount != 0 || status.ContainerID == "" || status.LastTerminationState.Terminated != nil || term == nil || term.ExitCode != 0 || term.FinishedAt.IsZero() || term.StartedAt.IsZero() || !matchesPinnedImageID(c.Image, status.ImageID) {
		return nil, ErrBinding
	}
	node, err := client.CoreV1().Nodes().Get(ctx, spec.NodeName, metav1.GetOptions{})
	if err != nil || node.UID == "" || node.DeletionTimestamp != nil {
		return nil, ErrBinding
	}
	mount, relative, err := effectiveMount(c, binding.RootPath)
	if err != nil || !mount.ReadOnly {
		return nil, ErrBinding
	}
	observed, err := inspectVolumeMode(ctx, client, pod, mount.Name, relative, true)
	if err != nil || observed.VolumeUID != binding.VolumeUID {
		return nil, ErrBinding
	}
	if settings.IncludePackages {
		if err := verifyPackageInventoryMount(ctx, client, pod); err != nil {
			return nil, err
		}
	}
	stream, err := read(ctx, job.Namespace, pod.Name, c.Name)
	if err != nil {
		return nil, ErrBinding
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, inventoryReportLimit+1))
	if err != nil || len(data) > inventoryReportLimit {
		return nil, ErrBinding
	}
	var report struct {
		Version    int       `json:"version"`
		ScanID     string    `json:"scanId"`
		Region     string    `json:"region"`
		Node       string    `json:"node"`
		NodeUID    string    `json:"nodeUid"`
		ObservedAt time.Time `json:"observedAt"`
	}
	if json.Unmarshal(data, &report) != nil || report.Version != 1 || report.ScanID != settings.ScanID || report.Region != settings.Region || report.Node != node.Name || report.NodeUID != string(node.UID) || report.ObservedAt.Before(term.StartedAt.Add(-5*time.Second)) || report.ObservedAt.After(term.FinishedAt.Add(5*time.Second)) {
		return nil, ErrBinding
	}
	current, err := client.CoreV1().Pods(job.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil || current.UID != pod.UID || current.ResourceVersion != pod.ResourceVersion {
		return nil, ErrBinding
	}
	currentJob, err := client.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
	if err != nil || currentJob.UID != job.UID || currentJob.ResourceVersion != job.ResourceVersion {
		return nil, ErrBinding
	}
	return json.RawMessage(data), nil
}
