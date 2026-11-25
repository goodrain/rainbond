package vmstorage

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	v1 "kubevirt.io/api/core/v1"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func readyPod(name string, containers ...corev1.Container) *corev1.Pod {
	p := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "infra"}, Spec: corev1.PodSpec{Containers: containers}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
	for _, c := range containers {
		p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, corev1.ContainerStatus{Name: c.Name, Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}})
	}
	return p
}
func fixture() (*kubefake.Clientset, *fake.FakeDynamicClient) {
	core := kubefake.NewSimpleClientset(&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "fast"}, Provisioner: "example.csi.io"}, &storagev1.CSIDriver{ObjectMeta: metav1.ObjectMeta{Name: "example.csi.io"}}, readyPod("controller", corev1.Container{Name: "snapshot-controller", Image: "registry/snapshot-controller:v1"}), readyPod("provider", corev1.Container{Name: "plugin", Args: []string{"--drivername=$(DRIVER_NAME)"}, Env: []corev1.EnvVar{{Name: "DRIVER_NAME", Value: "example.csi.io"}}}, corev1.Container{Name: "snapshotter", Image: "registry/csi-snapshotter:v1"}))
	kv := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "KubeVirt", "metadata": map[string]interface{}{"name": "virt", "namespace": "infra"}, "status": map[string]interface{}{"phase": "Deployed", "conditions": []interface{}{map[string]interface{}{"type": "Available", "status": "True"}, map[string]interface{}{"type": "Degraded", "status": "False"}}}}}
	class := snapshotClass("class-one", "example.csi.io", false)
	dynamic := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{snapshotClassGVR: "VolumeSnapshotClassList", volumeSnapshotGVR: "VolumeSnapshotList", vmSnapshotGVR: "VirtualMachineSnapshotList", kubeVirtGVR: "KubeVirtList"}, kv, class)
	return core, dynamic
}
func snapshotClass(name, driver string, def bool) *unstructured.Unstructured {
	a := map[string]interface{}{}
	if def {
		a["snapshot.storage.kubernetes.io/is-default-class"] = "true"
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "snapshot.storage.k8s.io/v1", "kind": "VolumeSnapshotClass", "metadata": map[string]interface{}{"name": name, "annotations": a}, "driver": driver, "deletionPolicy": "Delete"}}
}

// capability_id: rainbond.vm-storage.snapshot-capability
func TestSnapshotCapabilityUsesDriverClassAndReadiness(t *testing.T) {
	cases := []struct {
		name, status, reason string
		change               func(*kubefake.Clientset, *fake.FakeDynamicClient)
	}{
		{"ready", "supported", "ready", nil},
		{"class driver mismatch", "unsupported", "snapshot_class_missing", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			_, _ = d.Resource(snapshotClassGVR).Update(context.Background(), snapshotClass("class-one", "other.csi.io", false), metav1.UpdateOptions{})
		}},
		{"not a CSI driver", "unsupported", "csi_driver_not_registered", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			_ = c.StorageV1().CSIDrivers().Delete(context.Background(), "example.csi.io", metav1.DeleteOptions{})
		}},
		{"ambiguous classes", "unknown", "snapshot_class_ambiguous", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			_, _ = d.Resource(snapshotClassGVR).Create(context.Background(), snapshotClass("class-two", "example.csi.io", false), metav1.CreateOptions{})
		}},
		{"snapshot api forbidden", "unknown", "snapshot_class_read_failed", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			d.PrependReactor("list", "volumesnapshotclasses", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, fmt.Errorf("denied") })
		}},
		{"storage missing", "unknown", "storage_class_read_failed", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			_ = c.StorageV1().StorageClasses().Delete(context.Background(), "fast", metav1.DeleteOptions{})
		}},
		{"controller down", "unknown", "snapshot_controller_not_ready", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			_ = c.CoreV1().Pods("infra").Delete(context.Background(), "controller", metav1.DeleteOptions{})
		}},
		{"driver snapshotter down", "unknown", "driver_snapshotter_not_ready", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			p, _ := c.CoreV1().Pods("infra").Get(context.Background(), "provider", metav1.GetOptions{})
			p.Status.ContainerStatuses[1].Ready = false
			_, _ = c.CoreV1().Pods("infra").Update(context.Background(), p, metav1.UpdateOptions{})
		}},
		{"KubeVirt unreadable", "unknown", "kubevirt_read_failed", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			d.PrependReactor("list", "kubevirts", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, fmt.Errorf("denied") })
		}},
		{"KubeVirt not deployed", "unknown", "kubevirt_not_ready", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			v, _ := d.Resource(kubeVirtGVR).Namespace("infra").Get(context.Background(), "virt", metav1.GetOptions{})
			_ = unstructured.SetNestedField(v.Object, "Deploying", "status", "phase")
			_, _ = d.Resource(kubeVirtGVR).Namespace("infra").Update(context.Background(), v, metav1.UpdateOptions{})
		}},
		{"pod read forbidden", "unknown", "snapshot_services_read_failed", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			c.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, fmt.Errorf("denied") })
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c, d := fixture()
			if tt.change != nil {
				tt.change(c, d)
			}
			got := NewInspector(c, d).StorageClass(context.Background(), "fast")
			if got.Status != tt.status || got.ReasonCode != tt.reason || got.Supported != (tt.status == "supported") {
				t.Fatalf("got %+v, want %s/%s", got, tt.status, tt.reason)
			}
		})
	}
}

// capability_id: rainbond.vm-storage.snapshot-capability
func TestSnapshotCapabilityAssociatesSeparateSidecarsBySharedSocket(t *testing.T) {
	c, d := fixture()
	if err := c.CoreV1().Pods("infra").Delete(context.Background(), "provider", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	socket := func(name, path string, container corev1.Container) *corev1.Pod {
		container.VolumeMounts = []corev1.VolumeMount{{Name: "socket-dir", MountPath: "/csi/"}}
		pod := readyPod(name, container)
		pod.Spec.NodeName = "node-a"
		pod.Spec.Volumes = []corev1.Volume{{Name: "socket-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: path}}}}
		return pod
	}
	driver := socket("driver", "/var/lib/kubelet/plugins/example.csi.io", corev1.Container{Name: "plugin", Args: []string{"--drivername=example.csi.io", "--endpoint=unix:///csi/csi.sock"}})
	snapshotter := socket("snapshotter", "/var/lib/kubelet/plugins/example.csi.io", corev1.Container{Name: "snapshotter", Image: "registry/csi-snapshotter:v1", Args: []string{"--csi-address=/csi/csi.sock"}})
	for _, pod := range []*corev1.Pod{driver, snapshotter} {
		if _, err := c.CoreV1().Pods("infra").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	got := NewInspector(c, d).StorageClass(context.Background(), "fast")
	if !got.Supported || got.ReasonCode != "ready" {
		t.Fatalf("shared CSI socket was not accepted: %+v", got)
	}

	snapshotter.Spec.Volumes[0].HostPath.Path = "/var/lib/kubelet/plugins/other.csi.io"
	if _, err := c.CoreV1().Pods("infra").Update(context.Background(), snapshotter, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	got = NewInspector(c, d).StorageClass(context.Background(), "fast")
	if got.Supported || got.ReasonCode != "driver_snapshotter_not_ready" {
		t.Fatalf("unrelated CSI sockets were associated: %+v", got)
	}
}

// capability_id: rainbond.vm-storage.snapshot-socket-node
func TestSnapshotCapabilityNeverJoinsHostSocketsAcrossNodes(t *testing.T) {
	for _, tt := range []struct {
		name, driverNode, snapshotterNode string
		supported                         bool
	}{
		{"same node", "node-a", "node-a", true},
		{"different nodes", "node-a", "node-b", false},
		{"driver unscheduled", "", "node-a", false},
		{"snapshotter unscheduled", "node-a", "", false},
		{"both unscheduled", "", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			core, dyn := fixture()
			_ = core.CoreV1().Pods("infra").Delete(ctx, "provider", metav1.DeleteOptions{})
			for n, container := range []corev1.Container{
				{Name: "driver", Args: []string{"--drivername=example.csi.io", "--endpoint=unix:///csi/csi.sock"}},
				{Name: "snapshotter", Image: "registry/csi-snapshotter:v1", Args: []string{"--csi-address=/csi/csi.sock"}},
			} {
				container.VolumeMounts = []corev1.VolumeMount{{Name: "socket", MountPath: "/csi"}}
				pod := readyPod(container.Name, container)
				pod.Spec.NodeName = []string{tt.driverNode, tt.snapshotterNode}[n]
				pod.Spec.Volumes = []corev1.Volume{{Name: "socket", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/csi"}}}}
				if _, err := core.CoreV1().Pods("infra").Create(ctx, pod, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			got := NewInspector(core, dyn).StorageClass(ctx, "fast")
			if got.Supported != tt.supported || !tt.supported && got.ReasonCode != "driver_snapshotter_not_ready" {
				t.Fatalf("node/socket association: %+v", got)
			}
		})
	}
}

// capability_id: rainbond.vm-storage.snapshot-all-disks
func TestSnapshotCapabilityRequiresEveryParticipatingDisk(t *testing.T) {
	c, d := fixture()
	_, _ = c.StorageV1().StorageClasses().Create(context.Background(), &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Provisioner: "not-a-csi-driver"}, metav1.CreateOptions{})
	got := NewInspector(c, d).Volumes(context.Background(), []Volume{{Key: "root", StorageClass: "fast"}, {Key: "data", StorageClass: "other"}})
	if got.Supported || got.Status != "unsupported" || len(got.Volumes) != 2 || got.Volumes[1].ReasonCode != "csi_driver_not_registered" {
		t.Fatalf("partial snapshot permitted: %+v", got)
	}
	empty := NewInspector(c, d).Volumes(context.Background(), nil)
	if empty.Supported || empty.Status != "unknown" {
		t.Fatalf("empty disk set supported: %+v", empty)
	}
}

func boundVMFixture(t *testing.T) (*kubefake.Clientset, *fake.FakeDynamicClient, *v1.VirtualMachine) {
	t.Helper()
	c, d := fixture()
	sc := "fast"
	uid := types.UID("pvc-root")
	_, err := c.CoreV1().PersistentVolumeClaims("tenant").Create(context.Background(), &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: "tenant", UID: uid}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &sc, VolumeName: "pv-root"}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.CoreV1().PersistentVolumes().Create(context.Background(), &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv-root"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Name: "root", Namespace: "tenant", UID: uid}, PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: "example.csi.io", VolumeHandle: "handle-root"}}}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	vm := &v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: "tenant", UID: "vm-uid"}, Spec: v1.VirtualMachineSpec{Template: &v1.VirtualMachineInstanceTemplateSpec{Spec: v1.VirtualMachineInstanceSpec{Volumes: []v1.Volume{{Name: "root", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "root"}}}}}}}}}
	return c, d, vm
}

// capability_id: rainbond.vm-storage.snapshot-all-disks
func TestVirtualMachineSnapshotInspectsRealPVAndEveryDisk(t *testing.T) {
	t.Run("unrecognized source is never silently omitted", func(t *testing.T) {
		c, d, v := boundVMFixture(t)
		v.Spec.Template.Spec.Volumes = append(v.Spec.Template.Spec.Volumes, v1.Volume{Name: "data"})
		got := NewInspector(c, d).VirtualMachine(context.Background(), v)
		if got.Supported || got.Status != "unknown" || len(got.Volumes) != 2 {
			t.Fatalf("unknown participant omitted: %+v", got)
		}
	})
	t.Run("known auxiliary media excluded explicitly", func(t *testing.T) {
		c, d, v := boundVMFixture(t)
		v.Spec.Template.Spec.Volumes = append(v.Spec.Template.Spec.Volumes, v1.Volume{Name: "config", VolumeSource: v1.VolumeSource{ConfigMap: &v1.ConfigMapVolumeSource{}}})
		got := NewInspector(c, d).VirtualMachine(context.Background(), v)
		if !got.Supported || len(got.Volumes) != 1 {
			t.Fatalf("auxiliary media misclassified: %+v", got)
		}
	})

	t.Run("actual CSI backing", func(t *testing.T) {
		c, d, v := boundVMFixture(t)
		got := NewInspector(c, d).VirtualMachine(context.Background(), v)
		if !got.Supported || len(got.Volumes) != 1 {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("SC same name cannot hide another PV driver", func(t *testing.T) {
		c, d, v := boundVMFixture(t)
		p, _ := c.CoreV1().PersistentVolumes().Get(context.Background(), "pv-root", metav1.GetOptions{})
		p.Spec.CSI.Driver = "other.csi.io"
		_, _ = c.CoreV1().PersistentVolumes().Update(context.Background(), p, metav1.UpdateOptions{})
		got := NewInspector(c, d).VirtualMachine(context.Background(), v)
		if got.Supported || got.Volumes[0].ReasonCode != "pv_driver_mismatch" {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("PVC UID cannot claim foreign PV", func(t *testing.T) {
		c, d, v := boundVMFixture(t)
		p, _ := c.CoreV1().PersistentVolumes().Get(context.Background(), "pv-root", metav1.GetOptions{})
		p.Spec.ClaimRef.UID = "other-uid"
		_, _ = c.CoreV1().PersistentVolumes().Update(context.Background(), p, metav1.UpdateOptions{})
		got := NewInspector(c, d).VirtualMachine(context.Background(), v)
		if got.Supported || got.Volumes[0].ReasonCode != "pv_claim_mismatch" {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("unsupported data disk is not silently omitted", func(t *testing.T) {
		c, d, v := boundVMFixture(t)
		v.Spec.Template.Spec.Volumes = append(v.Spec.Template.Spec.Volumes, v1.Volume{Name: "data", VolumeSource: v1.VolumeSource{EmptyDisk: &v1.EmptyDiskSource{}}})
		got := NewInspector(c, d).VirtualMachine(context.Background(), v)
		if got.Supported || got.Status != "unsupported" || len(got.Volumes) != 2 {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("missing data volume never falls back to another PVC", func(t *testing.T) {
		c, d, v := boundVMFixture(t)
		v.Spec.Template.Spec.Volumes = append(v.Spec.Template.Spec.Volumes, v1.Volume{Name: "data", VolumeSource: v1.VolumeSource{DataVolume: &v1.DataVolumeSource{Name: "missing"}}})
		got := NewInspector(c, d).VirtualMachine(context.Background(), v)
		if got.Supported || got.Status != "unknown" || len(got.Volumes) != 2 {
			t.Fatalf("%+v", got)
		}
	})
}

// capability_id: rainbond.vm-storage.snapshot-capability
func TestSnapshotCapabilitySelectionAndFailedEvidence(t *testing.T) {
	for _, tt := range []struct {
		name      string
		change    func(*kubefake.Clientset, *fake.FakeDynamicClient)
		supported bool
		reason    string
	}{
		{"one default", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			_, _ = d.Resource(snapshotClassGVR).Create(context.Background(), snapshotClass("default", "example.csi.io", true), metav1.CreateOptions{})
		}, true, "ready"},
		{"two defaults", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			for _, n := range []string{"default1", "default2"} {
				_, _ = d.Resource(snapshotClassGVR).Create(context.Background(), snapshotClass(n, "example.csi.io", true), metav1.CreateOptions{})
			}
		}, false, "snapshot_class_ambiguous"},
		{"CSI read denied", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			c.PrependReactor("get", "csidrivers", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, fmt.Errorf("denied") })
		}, false, "csi_driver_read_failed"},
		{"CSI snapshot API denied", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			d.PrependReactor("list", "volumesnapshots", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, fmt.Errorf("denied") })
		}, false, "snapshot_api_read_failed"},
		{"missing KubeVirt", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			_ = d.Resource(kubeVirtGVR).Namespace("infra").Delete(context.Background(), "virt", metav1.DeleteOptions{})
		}, false, "kubevirt_not_ready"},
		{"missing env proof", func(c *kubefake.Clientset, d *fake.FakeDynamicClient) {
			p, _ := c.CoreV1().Pods("infra").Get(context.Background(), "provider", metav1.GetOptions{})
			p.Spec.Containers[0].Env = nil
			_, _ = c.CoreV1().Pods("infra").Update(context.Background(), p, metav1.UpdateOptions{})
		}, false, "driver_snapshotter_not_ready"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, d := fixture()
			tt.change(c, d)
			got := NewInspector(c, d).StorageClass(context.Background(), "fast")
			if got.Supported != tt.supported || got.ReasonCode != tt.reason {
				t.Fatalf("%+v", got)
			}
		})
	}
	if got := NewInspector(nil, nil).StorageClass(context.Background(), "fast"); got.Supported || got.Status != "unknown" {
		t.Fatal(got)
	}
	c, d := fixture()
	if got := NewInspector(c, d).StorageClass(context.Background(), ""); got.Supported || got.ReasonCode != "storage_class_missing" {
		t.Fatal(got)
	}
	if got := NewInspector(c, d).Volumes(context.Background(), []Volume{{Key: "duplicate", StorageClass: "fast"}, {Key: "duplicate", StorageClass: "fast"}}); got.Supported {
		t.Fatal("duplicate participant accepted")
	}
}

// capability_id: rainbond.vm-storage.snapshot-all-disks
func TestVirtualMachineSnapshotFailsClosedForUnreadyDisks(t *testing.T) {
	for _, tt := range []struct {
		name, reason string
		change       func(*kubefake.Clientset, *fake.FakeDynamicClient, *v1.VirtualMachine)
	}{
		{"missing PVC", "pvc_read_failed", func(c *kubefake.Clientset, d *fake.FakeDynamicClient, v *v1.VirtualMachine) {
			_ = c.CoreV1().PersistentVolumeClaims("tenant").Delete(context.Background(), "root", metav1.DeleteOptions{})
		}},
		{"pending PVC", "pvc_not_ready", func(c *kubefake.Clientset, d *fake.FakeDynamicClient, v *v1.VirtualMachine) {
			p, _ := c.CoreV1().PersistentVolumeClaims("tenant").Get(context.Background(), "root", metav1.GetOptions{})
			p.Status.Phase = corev1.ClaimPending
			_, _ = c.CoreV1().PersistentVolumeClaims("tenant").Update(context.Background(), p, metav1.UpdateOptions{})
		}},
		{"missing SC", "storage_class_missing", func(c *kubefake.Clientset, d *fake.FakeDynamicClient, v *v1.VirtualMachine) {
			p, _ := c.CoreV1().PersistentVolumeClaims("tenant").Get(context.Background(), "root", metav1.GetOptions{})
			p.Spec.StorageClassName = nil
			_, _ = c.CoreV1().PersistentVolumeClaims("tenant").Update(context.Background(), p, metav1.UpdateOptions{})
		}},
		{"missing PV", "pv_read_failed", func(c *kubefake.Clientset, d *fake.FakeDynamicClient, v *v1.VirtualMachine) {
			_ = c.CoreV1().PersistentVolumes().Delete(context.Background(), "pv-root", metav1.DeleteOptions{})
		}},
		{"nonCSI PV", "pv_not_csi", func(c *kubefake.Clientset, d *fake.FakeDynamicClient, v *v1.VirtualMachine) {
			p, _ := c.CoreV1().PersistentVolumes().Get(context.Background(), "pv-root", metav1.GetOptions{})
			p.Spec.CSI = nil
			_, _ = c.CoreV1().PersistentVolumes().Update(context.Background(), p, metav1.UpdateOptions{})
		}},
		{"missing handle", "pv_handle_missing", func(c *kubefake.Clientset, d *fake.FakeDynamicClient, v *v1.VirtualMachine) {
			p, _ := c.CoreV1().PersistentVolumes().Get(context.Background(), "pv-root", metav1.GetOptions{})
			p.Spec.CSI.VolumeHandle = ""
			_, _ = c.CoreV1().PersistentVolumes().Update(context.Background(), p, metav1.UpdateOptions{})
		}},
		{"DV with exact owner", "ready", func(c *kubefake.Clientset, d *fake.FakeDynamicClient, v *v1.VirtualMachine) {
			dv := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"name": "root", "namespace": "tenant", "uid": "dv-uid"}}}
			_, _ = d.Resource(schema.GroupVersionResource{Group: "cdi.kubevirt.io", Version: "v1beta1", Resource: "datavolumes"}).Namespace("tenant").Create(context.Background(), dv, metav1.CreateOptions{})
			v.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim = nil
			v.Spec.Template.Spec.Volumes[0].DataVolume = &v1.DataVolumeSource{Name: "root"}
			p, _ := c.CoreV1().PersistentVolumeClaims("tenant").Get(context.Background(), "root", metav1.GetOptions{})
			p.OwnerReferences = []metav1.OwnerReference{{Kind: "DataVolume", UID: "dv-uid"}}
			_, _ = c.CoreV1().PersistentVolumeClaims("tenant").Update(context.Background(), p, metav1.UpdateOptions{})
		}},
		{"DV wrong owner", "data_volume_claim_unverified", func(c *kubefake.Clientset, d *fake.FakeDynamicClient, v *v1.VirtualMachine) {
			dv := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"name": "root", "namespace": "tenant", "uid": "dv-uid"}}}
			_, _ = d.Resource(schema.GroupVersionResource{Group: "cdi.kubevirt.io", Version: "v1beta1", Resource: "datavolumes"}).Namespace("tenant").Create(context.Background(), dv, metav1.CreateOptions{})
			v.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim = nil
			v.Spec.Template.Spec.Volumes[0].DataVolume = &v1.DataVolumeSource{Name: "root"}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, d, v := boundVMFixture(t)
			tt.change(c, d, v)
			got := NewInspector(c, d).VirtualMachine(context.Background(), v)
			if len(got.Volumes) != 1 || got.Volumes[0].ReasonCode != tt.reason || got.Supported != (tt.reason == "ready") {
				t.Fatalf("%+v", got)
			}
		})
	}
	c, d, v := boundVMFixture(t)
	if NewInspector(c, d).VirtualMachine(context.Background(), nil).Supported {
		t.Fatal("nil VM accepted")
	}
	v.Spec.Template = nil
	if NewInspector(c, d).VirtualMachine(context.Background(), v).Supported {
		t.Fatal("nil template accepted")
	}
	if (&UnavailableError{Capability: SnapshotSet{Capability: unknown("unavailable", "read failed")}}).Error() == "" {
		t.Fatal("empty rejection")
	}
}

// capability_id: rainbond.vm-storage.snapshot-capability
func TestSnapshotCapabilityTypedNilClientsStayUnknown(t *testing.T) {
	var core *kubefake.Clientset
	_, dyn := fixture()
	got := NewInspector(core, dyn).StorageClass(context.Background(), "fast")
	if got.Supported || got.Status != "unknown" {
		t.Fatalf("%+v", got)
	}
}

// capability_id: rainbond.vm-storage.snapshot-capability
func TestStorageProfileSnapshotClassTakesPrecedence(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "cdi.kubevirt.io", Version: "v1beta1", Resource: "storageprofiles"}
	for _, tt := range []struct {
		name, preferred, reason string
		deny                    bool
	}{
		{"explicit profile", "preferred", "ready", false}, {"dangling profile", "missing", "storage_profile_class_unverified", false}, {"profile read denied", "", "storage_profile_read_failed", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, d := fixture()
			_, _ = d.Resource(snapshotClassGVR).Create(context.Background(), snapshotClass("preferred", "example.csi.io", false), metav1.CreateOptions{})
			_, _ = d.Resource(gvr).Create(context.Background(), &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "StorageProfile", "metadata": map[string]interface{}{"name": "fast"}, "status": map[string]interface{}{"snapshotClass": tt.preferred}}}, metav1.CreateOptions{})
			if tt.deny {
				d.PrependReactor("get", "storageprofiles", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, fmt.Errorf("denied") })
			}
			got := NewInspector(c, d).StorageClass(context.Background(), "fast")
			if got.ReasonCode != tt.reason || got.Supported != (tt.reason == "ready") {
				t.Fatalf("%+v", got)
			}
			if got.Supported && got.VolumeSnapshotClass != "preferred" {
				t.Fatal("profile ignored")
			}
		})
	}
}
