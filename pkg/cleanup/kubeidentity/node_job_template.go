package kubeidentity

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
)

// NodeJobSettings comes exclusively from the operator, never a deletion request.
// Direct Core access mounts mTLS files; signed Console access projects only key.
type NodeJobSettings struct {
	Region, Image, Endpoint, CredentialSecret, StateClaim string
	ConsoleEnterprise                                     string
	AllowConsoleHTTP                                      bool
}

var nodeConsoleScope = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// BuildManagedNodeJob mounts the observed system builder cache and a separate
// persistent journal. It constructs a suspended single-execution Job, not a grant.
func BuildManagedNodeJob(ctx context.Context, client kubernetes.Interface, sourcePod, sourceUID string, binding coordination.StorageRegistration, request coordination.CoordinationRequest, intent coordination.NodeJobIntent, settings NodeJobSettings) (*batchv1.Job, error) {
	expected, err := coordination.ManagedNodeRequest(binding, request.Owner, request.OperationID, request.Fingerprint, intent)
	if err != nil || expected != request || settings.Region == "" || len(settings.Region) > 63 || len(validation.IsDNS1123Subdomain(settings.CredentialSecret)) != 0 || len(validation.IsDNS1123Subdomain(settings.StateClaim)) != 0 {
		return nil, ErrBinding
	}
	parts := strings.Split(settings.Image, "@sha256:")
	if len(parts) != 2 || parts[0] == "" || len(parts[1]) != 64 {
		return nil, ErrBinding
	}
	digest, err := hex.DecodeString(parts[1])
	if err != nil || hex.EncodeToString(digest) != parts[1] {
		return nil, ErrBinding
	}
	endpoint, err := url.Parse(settings.Endpoint)
	if err != nil || (endpoint.Scheme != "https" && !(settings.ConsoleEnterprise != "" && settings.AllowConsoleHTTP && endpoint.Scheme == "http")) || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") {
		return nil, ErrBinding
	}
	if settings.ConsoleEnterprise != "" && (!nodeConsoleScope.MatchString(settings.ConsoleEnterprise) || !nodeConsoleScope.MatchString(settings.Region)) {
		return nil, ErrBinding
	}
	observed, err := InspectManagedBuildCache(ctx, client, intent.Namespace, sourcePod, sourceUID)
	if err != nil || observed.NodeUID != intent.NodeUID || observed.Mount.NodeName != intent.NodeName || observed.Mount.VolumeUID != binding.VolumeUID {
		return nil, ErrBinding
	}
	pod, err := client.CoreV1().Pods(intent.Namespace).Get(ctx, sourcePod, metav1.GetOptions{})
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
	state, err := client.CoreV1().PersistentVolumeClaims(intent.Namespace).Get(ctx, settings.StateClaim, metav1.GetOptions{})
	if err != nil || state.UID == "" || state.DeletionTimestamp != nil || state.Status.Phase != corev1.ClaimBound || state.Spec.VolumeName == "" {
		return nil, ErrBinding
	}
	if data.PersistentVolumeClaim != nil && data.PersistentVolumeClaim.ClaimName == settings.StateClaim {
		return nil, ErrBinding
	}
	// Pin the actual journal PV, not merely a PVC name that can be rebound.
	statePV, err := client.CoreV1().PersistentVolumes().Get(ctx, state.Spec.VolumeName, metav1.GetOptions{})
	if err != nil || statePV.UID == "" || statePV.DeletionTimestamp != nil || statePV.Spec.ClaimRef == nil || statePV.Spec.ClaimRef.UID != state.UID || statePV.Spec.ClaimRef.Namespace != intent.Namespace || statePV.Spec.ClaimRef.Name != state.Name {
		return nil, ErrBinding
	}
	data.Name = "node-cache"
	mount.Name = data.Name
	mount.MountPath = "/cache/build"
	mount.SubPath = relative
	descriptor := nodeTaskDescriptor(settings.Region, request, intent)
	raw, err := json.Marshal(descriptor)
	if err != nil {
		return nil, ErrBinding
	}
	no, yes := false, true
	zero, one := int32(0), int32(1)
	root := int64(0)
	mode := int32(0400)
	args := []string{"--core-endpoint=" + settings.Endpoint, "--credential-file=/node-control/token", "--ca-file=/node-control/ca.crt", "--client-cert-file=/node-control/tls.crt", "--client-key-file=/node-control/tls.key"}
	control := &corev1.SecretVolumeSource{SecretName: settings.CredentialSecret, DefaultMode: &mode}
	if settings.ConsoleEnterprise != "" {
		args = []string{"--core-endpoint=" + settings.Endpoint, "--console-enterprise=" + settings.ConsoleEnterprise, "--credential-file=/node-control/key"}
		if settings.AllowConsoleHTTP {
			args = append(args, "--allow-internal-http")
		}
		control.Items = []corev1.KeyToPath{{Key: "key", Path: "key"}}
	}
	c := corev1.Container{Name: "node-cleanup", Image: settings.Image, Command: []string{"/app/node-cleanup"}, Args: args, Env: []corev1.EnvVar{{Name: "CLEANUP_NODE_OPERATION_BASE64", Value: base64.StdEncoding.EncodeToString(raw)}, {Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.name"}}}, {Name: "POD_UID", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.uid"}}}}, VolumeMounts: []corev1.VolumeMount{mount, {Name: "node-state", MountPath: "/node-state"}, {Name: "node-control", MountPath: "/node-control", ReadOnly: true}}, SecurityContext: &corev1.SecurityContext{RunAsUser: &root, RunAsGroup: &root, AllowPrivilegeEscalation: &no, ReadOnlyRootFilesystem: &yes, Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}}}
	// No service-account token or inherited builder environment/credentials.
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: intent.Namespace}, Spec: batchv1.JobSpec{Suspend: &yes, BackoffLimit: &zero, Completions: &one, Parallelism: &one, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{NodeName: intent.NodeName, RestartPolicy: corev1.RestartPolicyNever, AutomountServiceAccountToken: &no, ImagePullSecrets: append([]corev1.LocalObjectReference(nil), pod.Spec.ImagePullSecrets...), Containers: []corev1.Container{c}, Volumes: []corev1.Volume{*data, {Name: "node-state", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: settings.StateClaim}}}, {Name: "node-control", VolumeSource: corev1.VolumeSource{Secret: control}}}}}}}
	source, _ := json.Marshal(struct {
		IDs  []string
		Spec batchv1.JobSpec
	}{[]string{sourceUID, observed.Mount.VolumeUID, string(state.UID), string(statePV.UID)}, job.Spec})
	sum := sha256.Sum256(source)
	job.Spec.Template.Annotations = map[string]string{nodeJournalPVCUID: string(state.UID), nodeJournalPVUID: string(statePV.UID), "rainbond.io/node-source-fingerprint": hex.EncodeToString(sum[:]), "rainbond.io/node-source-pod": sourcePod, "rainbond.io/node-source-uid": sourceUID}
	return job, nil
}
func nodeTaskDescriptor(region string, r coordination.CoordinationRequest, intent coordination.NodeJobIntent) map[string]interface{} {
	id := sha256.Sum256([]byte(region + "/" + r.StorageID + "\x00cache\x00" + intent.Entry))
	target := map[string]interface{}{"nodeUid": intent.NodeUID, "storageId": r.StorageID, "entry": intent.Entry, "fingerprint": intent.Fingerprint}
	return map[string]interface{}{"protocol": 1, "generation": r.Generation, "owner": r.Owner, "operationId": r.OperationID, "fingerprint": r.Fingerprint, "stateDir": "/node-state", "config": map[string]interface{}{"region": region, "node": intent.NodeName, "nodeUid": intent.NodeUID, "roots": []interface{}{map[string]interface{}{"kind": "cache", "storageId": r.StorageID, "path": "/cache/build"}}}, "resource": map[string]interface{}{"id": hex.EncodeToString(id[:]), "cluster": region, "category": "nodeResources", "resourceType": "cache", "node": intent.NodeName, "nodeUid": intent.NodeUID, "name": intent.Entry, "owner": r.StorageID, "source": "node_build_cache", "decision": "direct", "managedTarget": target}}
}

// SameManagedNodeSource checks the freshly reconstructed source before startup.
func SameManagedNodeSource(original, current *batchv1.Job) bool {
	if original == nil || current == nil {
		return false
	}
	before := original.Spec.Template.Annotations["rainbond.io/node-source-fingerprint"]
	return len(before) == 64 && before == current.Spec.Template.Annotations["rainbond.io/node-source-fingerprint"]
}
