package k8s

import (
	"os"
	"testing"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// capability_id: rainbond.ingress-nginx.name-namespace-parse
func TestParseNameNS(t *testing.T) {
	ns, name, err := ParseNameNS("demo/app")
	if err != nil {
		t.Fatal(err)
	}
	if ns != "demo" || name != "app" {
		t.Fatalf("unexpected parse result: ns=%q name=%q", ns, name)
	}

	if _, _, err := ParseNameNS("invalid"); err == nil {
		t.Fatal("expected invalid format error")
	}
}

// capability_id: rainbond.ingress-nginx.meta-namespace-key
func TestMetaNamespaceKey(t *testing.T) {
	obj := &apiv1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}}
	if got := MetaNamespaceKey(obj); got != "default/demo" {
		t.Fatalf("unexpected meta namespace key: %q", got)
	}
}

// capability_id: rainbond.ingress-nginx.node-ip-resolve
func TestGetNodeIPOrName(t *testing.T) {
	client := fake.NewSimpleClientset(&apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Status: apiv1.NodeStatus{
			Addresses: []apiv1.NodeAddress{
				{Type: apiv1.NodeInternalIP, Address: "10.0.0.1"},
				{Type: apiv1.NodeExternalIP, Address: "1.1.1.1"},
			},
		},
	})
	if got := GetNodeIPOrName(client, "node-1", true); got != "10.0.0.1" {
		t.Fatalf("expected internal ip, got %q", got)
	}
	if got := GetNodeIPOrName(client, "node-1", false); got != "1.1.1.1" {
		t.Fatalf("expected external ip, got %q", got)
	}
}

// capability_id: rainbond.ingress-nginx.pod-details
func TestGetPodDetails(t *testing.T) {
	t.Setenv("POD_NAME", "demo-pod")
	t.Setenv("POD_NAMESPACE", "default")
	client := fake.NewSimpleClientset(
		&apiv1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
			Status: apiv1.NodeStatus{
				Addresses: []apiv1.NodeAddress{
					{Type: apiv1.NodeInternalIP, Address: "10.0.0.1"},
				},
			},
		},
		&apiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod",
				Namespace: "default",
				Labels:    map[string]string{"app": "demo"},
			},
			Spec: apiv1.PodSpec{NodeName: "node-1"},
		},
	)
	info, err := GetPodDetails(client)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "demo-pod" || info.Namespace != "default" || info.NodeIP != "10.0.0.1" {
		t.Fatalf("unexpected pod info: %+v", info)
	}

	os.Unsetenv("POD_NAME")
	if _, err := GetPodDetails(client); err == nil {
		t.Fatal("expected missing env error")
	}
}
