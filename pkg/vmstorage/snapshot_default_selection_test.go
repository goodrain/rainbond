package vmstorage

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// capability_id: rainbond.vm-storage.snapshot-capability
func TestSnapshotDefaultTruthMatrix(t *testing.T) {
	cases := []struct {
		name     string
		flags    []string
		profile  string
		expected string
		reason   string
	}{
		{"true plus explicit false", []string{"true", "false"}, "", "candidate-a", "ready"},
		{"false before true", []string{"false", "true"}, "", "candidate-b", "ready"},
		{"true plus absent", []string{"true", "absent"}, "", "candidate-a", "ready"},
		{"two true", []string{"true", "true"}, "", "", "snapshot_class_ambiguous"},
		{"false plus absent", []string{"false", "absent"}, "", "", "snapshot_class_ambiguous"},
		{"two absent", []string{"absent", "absent"}, "", "", "snapshot_class_ambiguous"},
		{"two false", []string{"false", "false"}, "", "", "snapshot_class_ambiguous"},
		{"one false but unique matching class", []string{"false"}, "", "candidate-a", "ready"},
		{"profile explicit false class wins", []string{"true", "false"}, "candidate-b", "candidate-b", "ready"},
		{"profile resolves multiple defaults", []string{"true", "true"}, "candidate-b", "candidate-b", "ready"},
		{"dangling profile never falls back", []string{"true", "false"}, "missing", "", "storage_profile_class_unverified"},
	}
	outputs := map[string]Capability{}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			c, d := fixture()
			if err := d.Resource(snapshotClassGVR).Delete(ctx, "class-one", metav1.DeleteOptions{}); err != nil {
				t.Fatal(err)
			}
			for n, flag := range tt.flags {
				name := "candidate-" + string(rune('a'+n))
				v := snapshotClass(name, "example.csi.io", false)
				if flag != "absent" {
					v.SetAnnotations(map[string]string{"snapshot.storage.kubernetes.io/is-default-class": flag})
				}
				if _, err := d.Resource(snapshotClassGVR).Create(ctx, v, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			// An unrelated driver's true default is never eligible for this StorageClass.
			if _, err := d.Resource(snapshotClassGVR).Create(ctx, snapshotClass("other-driver-default", "other.csi.io", true), metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			if tt.profile != "" {
				gvr := schema.GroupVersionResource{Group: "cdi.kubevirt.io", Version: "v1beta1", Resource: "storageprofiles"}
				_, err := d.Resource(gvr).Create(ctx, &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "StorageProfile", "metadata": map[string]interface{}{"name": "fast"}, "status": map[string]interface{}{"snapshotClass": tt.profile}}}, metav1.CreateOptions{})
				if err != nil {
					t.Fatal(err)
				}
			}
			got := NewInspector(c, d).StorageClass(ctx, "fast")
			outputs[tt.name] = got
			if got.Supported != (tt.expected != "") || got.VolumeSnapshotClass != tt.expected || got.ReasonCode != tt.reason {
				t.Fatalf("got %+v; want class=%s reason=%s", got, tt.expected, tt.reason)
			}
		})
	}
	if target := os.Getenv("DEFAULT_SELECTION_OBSERVATIONS"); target != "" {
		b, err := json.MarshalIndent(outputs, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(target, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
