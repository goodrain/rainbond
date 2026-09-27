package kubeidentity

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"strings"
	"testing"
)

// capability_id: rainbond.cleanup.node-inventory-template
func TestCacheInventoryTemplateUsesObservedReadOnlyStorage(t *testing.T) {
	_, _, pvc, pv := bindingObjects()
	pod := gcCleanerFixture()
	pod.Spec.NodeName = "node"
	pod.Spec.Volumes = []corev1.Volume{{Name: "cache", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}}
	pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "cache", MountPath: "/cache", SubPath: "owned"}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "node-uid"}}
	report := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "reports", Namespace: "system", UID: "report-uid"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "report-pv"}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	reportPV := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "report-pv", UID: "report-pv-uid"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Name: report.Name, Namespace: "system", UID: report.UID}}}
	client := fake.NewSimpleClientset(pod, pvc, pv, node, report, reportPV)
	volume := volumeIdentity("pvc", "system", string(pvc.UID), string(pv.UID), "owned/build")
	sum := sha256.Sum256([]byte("managed-build-cache\x00" + volume))
	binding := coordination.StorageRegistration{StorageID: hex.EncodeToString(sum[:]), Generation: "one", RootPath: "/cache/build", VolumeUID: volume}
	settings := NodeInventorySettings{Region: "rainbond", Image: "example.test/plugin@sha256:" + strings.Repeat("b", 64), ReportClaim: "reports"}
	job, err := BuildManagedCacheInventoryJob(context.Background(), client, "system", pod.Name, string(pod.UID), binding, settings)
	if err != nil {
		t.Fatal(err)
	}
	spec := job.Spec.Template.Spec
	c := spec.Containers[0]
	if !*job.Spec.Suspend || spec.NodeName != "node" || *spec.AutomountServiceAccountToken || len(spec.Volumes) != 2 || len(c.VolumeMounts) != 2 || !c.VolumeMounts[0].ReadOnly || c.VolumeMounts[0].SubPath != "owned/build" || !spec.Volumes[0].PersistentVolumeClaim.ReadOnly {
		t.Fatal("unbounded collector authority")
	}
	if c.Command[0] != "/app/node-inventory" || *c.SecurityContext.RunAsGroup != 10001 || spec.SecurityContext != nil {
		t.Fatal("invalid collector contract")
	}
	raw, err := base64.StdEncoding.DecodeString(c.Env[0].Value)
	var config struct {
		NodeUID string `json:"nodeUid"`
		Roots   []struct {
			StorageID string `json:"storageId"`
		} `json:"roots"`
	}
	if err != nil || json.Unmarshal(raw, &config) != nil || config.NodeUID != "node-uid" || len(config.Roots) != 1 || config.Roots[0].StorageID != binding.StorageID {
		t.Fatal("invented inventory identity")
	}
	if pod.Spec.Containers[0].VolumeMounts[0].ReadOnly {
		t.Fatal("source mutated")
	}
	settings.ReportClaim = pvc.Name
	if _, err := BuildManagedCacheInventoryJob(context.Background(), client, "system", pod.Name, string(pod.UID), binding, settings); err == nil {
		t.Fatal("report can write scanned volume")
	}
	settings.ReportClaim = "reports"
	binding.VolumeUID = "replaced"
	if _, err := BuildManagedCacheInventoryJob(context.Background(), client, "system", pod.Name, string(pod.UID), binding, settings); err == nil {
		t.Fatal("wrong source accepted")
	}
	for _, action := range client.Actions() {
		if action.GetResource().Resource == "secrets" {
			t.Fatal("read secrets")
		}
	}
}
