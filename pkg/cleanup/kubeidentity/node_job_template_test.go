package kubeidentity

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// capability_id: rainbond.cleanup.node-job-template
// capability_id: rainbond.cleanup.node-journal-identity
func TestNodeJobTemplateBindsCacheAndDurableState(t *testing.T) {
	_, _, pvc, pv := bindingObjects()
	pod := gcCleanerFixture()
	pod.Spec.NodeName = "node"
	pod.Spec.Volumes = []corev1.Volume{{Name: "cache", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}}
	pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "cache", MountPath: "/cache", SubPath: "owned"}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "node-uid"}}
	state := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "cleanup-state", Namespace: "system", UID: "state-uid"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "state-volume"}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	statePV := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "state-volume", UID: "state-pv-uid"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Name: state.Name, Namespace: "system", UID: state.UID}}}
	client := fake.NewSimpleClientset(pod, pvc, pv, node, state, statePV)
	volume := volumeIdentity("pvc", "system", string(pvc.UID), string(pv.UID), "owned/build")
	sum := sha256.Sum256([]byte("managed-build-cache\x00" + volume))
	binding := coordination.StorageRegistration{StorageID: hex.EncodeToString(sum[:]), Generation: "one", RootPath: "/cache/build", VolumeUID: volume}
	intent := coordination.NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "node-uid", Entry: "selected", Fingerprint: strings.Repeat("a", 64)}
	request, err := coordination.ManagedNodeRequest(binding, "owner", "operation", "plan", intent)
	if err != nil {
		t.Fatal(err)
	}
	settings := NodeJobSettings{Region: "rainbond", Image: "example.test/plugin@sha256:" + strings.Repeat("b", 64), Endpoint: "https://core.internal:8443", CredentialSecret: "cleanup-core", StateClaim: state.Name}
	job, err := BuildManagedNodeJob(context.Background(), client, pod.Name, string(pod.UID), binding, request, intent, settings)
	if err != nil {
		t.Fatal(err)
	}
	job.Name = "original-job"
	job.UID = "original-job-uid"
	intent.Name = job.Name
	intent.SpecHash, err = coordination.NodeJobSpecHash(job, intent)
	if err != nil {
		t.Fatal(err)
	}
	execution := coordination.NodeJobBinding{Protocol: 1, NodeJobIntent: intent, JobUID: string(job.UID)}
	if err := InspectNodeJournalVolume(context.Background(), client, job, execution); err != nil {
		t.Fatal("original journal not bound", err)
	}
	state.UID = "replacement-state"
	client.CoreV1().PersistentVolumeClaims("system").Update(context.Background(), state, metav1.UpdateOptions{})
	if err := InspectNodeJournalVolume(context.Background(), client, job, execution); err == nil {
		t.Fatal("replacement journal accepted")
	}
	state.UID = "state-uid"
	client.CoreV1().PersistentVolumeClaims("system").Update(context.Background(), state, metav1.UpdateOptions{})
	intent.Name = ""
	intent.SpecHash = ""
	c := job.Spec.Template.Spec.Containers[0]
	if !*job.Spec.Suspend || *job.Spec.BackoffLimit != 0 || job.Spec.Template.Spec.NodeName != "node" || c.Command[0] != "/app/node-cleanup" || c.VolumeMounts[0].SubPath != "owned/build" {
		t.Fatal("unsafe execution mapping")
	}
	data, err := base64.StdEncoding.DecodeString(c.Env[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	var descriptor map[string]interface{}
	if json.Unmarshal(data, &descriptor) != nil || descriptor["operationId"] != "operation" || descriptor["stateDir"] != "/node-state" {
		t.Fatal("wrong task descriptor")
	}
	for _, action := range client.Actions() {
		if action.GetResource().Resource == "secrets" {
			t.Fatal("read secret value")
		}
	}

	settings.ConsoleEnterprise = "enterprise"
	settings.Endpoint = "https://console.internal"
	signed, err := BuildManagedNodeJob(context.Background(), client, pod.Name, string(pod.UID), binding, request, intent, settings)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(signed.Spec.Template.Spec.Containers[0].Args, " ")
	if !strings.Contains(args, "--console-enterprise=enterprise") || strings.Contains(args, "client-cert-file") || strings.Contains(args, "/token") {
		t.Fatal("signed executor uses incorrect credentials", args)
	}
	secret := signed.Spec.Template.Spec.Volumes[2].Secret
	if len(secret.Items) != 1 || secret.Items[0].Key != "key" || secret.Items[0].Path != "key" {
		t.Fatal("signed executor did not project only installation key")
	}
	if SameManagedNodeSource(job, signed) {
		t.Fatal("authentication change did not invalidate source fingerprint")
	}
	settings.Endpoint = "http://console.internal"
	if _, err := BuildManagedNodeJob(context.Background(), client, pod.Name, string(pod.UID), binding, request, intent, settings); err == nil {
		t.Fatal("plaintext allowed without opt-in")
	}
	settings.AllowConsoleHTTP = true
	if _, err := BuildManagedNodeJob(context.Background(), client, pod.Name, string(pod.UID), binding, request, intent, settings); err != nil {
		t.Fatal(err)
	}
	settings.ConsoleEnterprise = "../other"
	if _, err := BuildManagedNodeJob(context.Background(), client, pod.Name, string(pod.UID), binding, request, intent, settings); err == nil {
		t.Fatal("invalid enterprise accepted")
	}
	settings.ConsoleEnterprise = ""
	if _, err := BuildManagedNodeJob(context.Background(), client, pod.Name, string(pod.UID), binding, request, intent, settings); err == nil {
		t.Fatal("Console opt-in weakened direct Core TLS")
	}
	settings.AllowConsoleHTTP = false
	settings.Endpoint = "https://core.internal:8443"
	settings.Image = "example.test/plugin:latest"
	if _, err := BuildManagedNodeJob(context.Background(), client, pod.Name, string(pod.UID), binding, request, intent, settings); err == nil {
		t.Fatal("mutable executor image accepted")
	}
	settings.Image = c.Image
	intent.Entry = "../other"
	if _, err := BuildManagedNodeJob(context.Background(), client, pod.Name, string(pod.UID), binding, request, intent, settings); err == nil {
		t.Fatal("changed selection accepted")
	}
	intent.Entry = "selected"
	state.Status.Phase = corev1.ClaimPending
	client.CoreV1().PersistentVolumeClaims("system").Update(context.Background(), state, metav1.UpdateOptions{})
	if _, err := BuildManagedNodeJob(context.Background(), client, pod.Name, string(pod.UID), binding, request, intent, settings); err == nil {
		t.Fatal("unbound durable state accepted")
	}
}
