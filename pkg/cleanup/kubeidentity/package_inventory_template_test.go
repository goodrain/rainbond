package kubeidentity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// capability_id: rainbond.cleanup.package-inventory-mount
func TestPackageInventoryUsesObservedReadOnlySubtree(t *testing.T) {
	pod := gcCleanerFixture()
	pod.Spec.NodeName = "node"
	pod.Spec.Volumes = []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/owned/grdata"}}}}
	pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "data", MountPath: "/grdata"}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "node-uid"}}
	client := fake.NewSimpleClientset(pod, node)
	volume, mount, roots, err := packageInventoryMount(context.Background(), client, pod)
	if err != nil {
		t.Fatal(err)
	}
	if volume.Name != "node-packages" || mount.Name != volume.Name || mount.MountPath != "/packages" || mount.SubPath != "package_build" || !mount.ReadOnly {
		t.Fatal("package mount is not a bounded read-only subtree")
	}
	if len(roots) != 2 || roots[0]["path"] != "/packages/temp/events" || roots[1]["path"] != "/packages/components" || roots[0]["storageId"] == "" {
		t.Fatal("incorrect package roots", roots)
	}
	if roots[0]["storageId"] == roots[1]["storageId"] {
		t.Fatal("temporary and retained package identity conflated")
	}
	raw, _ := json.Marshal(map[string]interface{}{"roots": roots})
	collector := pod.DeepCopy()
	collector.Spec.Volumes = []corev1.Volume{volume}
	collector.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{mount}
	collector.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "CLEANUP_NODE_INVENTORY_BASE64", Value: base64.StdEncoding.EncodeToString(raw)}}
	if err := verifyPackageInventoryMount(context.Background(), client, collector); err != nil {
		t.Fatal(err)
	}
	collector.Spec.Volumes[0].HostPath.Path = "/foreign/grdata"
	if err := verifyPackageInventoryMount(context.Background(), client, collector); err == nil {
		t.Fatal("replaced physical storage accepted")
	}
	pod.Spec.Containers[0].VolumeMounts[0].SubPathExpr = "$(UNTRUSTED)"
	if _, _, _, err := packageInventoryMount(context.Background(), client, pod); err == nil {
		t.Fatal("dynamic subpath accepted")
	}
	if strings.Contains(string(raw), "/owned/grdata") {
		t.Fatal("host path leaked into report configuration")
	}
}
