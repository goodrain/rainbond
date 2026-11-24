package handler

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	k8stesting "k8s.io/client-go/testing"
	kv "kubevirt.io/api/core/v1"
)

type deliveryCaptureManager struct {
	bindingConsumerDB
	tenant *dbmodel.Tenants
}
type deliveryCaptureTenantDAO struct {
	dbdao.TenantDao
	tenant *dbmodel.Tenants
}

func (m deliveryCaptureManager) TenantDao() dbdao.TenantDao {
	return deliveryCaptureTenantDAO{tenant: m.tenant}
}
func (d deliveryCaptureTenantDAO) GetTenantByUUID(string) (*dbmodel.Tenants, error) {
	return d.tenant, nil
}

func deliveryCaptureFixture(t *testing.T) (*vmDeliveryCapture, bindingConsumerFixture, string) {
	f := newBindingConsumerFixture(t)
	name := "delivery-" + strings.Repeat("a", 32)
	off := false
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "source", Namespace: f.scope.Namespace, UID: "vm-source", Labels: map[string]string{"service_id": f.scope.ServiceID}}, Spec: kv.VirtualMachineSpec{Running: &off, Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{Domain: kv.DomainSpec{Devices: kv.Devices{Disks: []kv.Disk{{Name: "root", DiskDevice: kv.DiskDevice{Disk: &kv.DiskTarget{}}}}}}, Volumes: []kv.Volume{{Name: "root", VolumeSource: kv.VolumeSource{PersistentVolumeClaim: &kv.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "active-a"}}}}}}}}}
	object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.dynamic.Resource(deliveryVMGVR).Namespace(f.scope.Namespace).Create(context.Background(), &unstructured.Unstructured{Object: object}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	f.dynamic.PrependReactor("create", "virtualmachineexports", func(action k8stesting.Action) (bool, runtime.Object, error) {
		value := action.(k8stesting.CreateAction).GetObject().(*unstructured.Unstructured)
		value.SetUID("export-source")
		value.SetResourceVersion("1")
		return false, nil, nil
	})
	capture := &vmDeliveryCapture{scope: f.scope, store: f.store, core: f.core, dynamic: f.dynamic, mq: f.mq, shareStatus: func(string) (string, error) { return "", nil }}
	return capture, f, name
}

// capability_id: rainbond.vm-delivery.current-system-disk
func TestVMDeliveryCaptureIsIdempotentAndReleasesOnlyAfterReadersGone(t *testing.T) {
	c, f, name := deliveryCaptureFixture(t)
	ctx := context.Background()
	if _, err := c.prepare(ctx, name); err != nil {
		t.Fatal(err)
	}
	if _, err := c.prepare(ctx, name); err != nil {
		t.Fatal("prepare did not replay", err)
	}
	export, err := f.dynamic.Resource(vmExportGVR).Namespace(f.scope.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	export.Object["status"] = map[string]interface{}{"phase": "Ready", "links": map[string]interface{}{"internal": map[string]interface{}{"volumes": []interface{}{map[string]interface{}{"formats": []interface{}{map[string]interface{}{"format": "gzip", "url": "https://virt-export-source.team.svc/volumes/root/disk.img.gz"}}}}}}}
	if _, err = f.dynamic.Resource(vmExportGVR).Namespace(f.scope.Namespace).Update(ctx, export, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	request := VMDeliveryRequest{Arch: "amd64"}
	request.ImageInfo.HubURL = "registry.test"
	request.ImageInfo.Namespace = "delivery"
	status, err := c.image(ctx, name, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.image(ctx, name, request); err != nil {
		t.Fatal(err)
	}
	if len(f.mq.tasks) != 1 {
		t.Fatal("replayed request enqueued another build")
	}
	raw, _ := json.Marshal(status)
	if strings.Contains(string(raw), "token") || strings.Contains(string(raw), "disk.img.gz") {
		t.Fatal("public status exposed export transport")
	}
	body, _ := json.Marshal(f.mq.tasks[0].TaskBody)
	if !strings.Contains(string(body), `"vm_capture_id":"`+name+`"`) {
		t.Fatal("build does not use capture identity")
	}
	c.shareStatus = func(string) (string, error) { return "success", nil }
	status, err = c.status(ctx, name)
	if err != nil || status.Phase != "success" {
		t.Fatal(status, err)
	}
	if _, err = f.core.CoreV1().Pods(f.scope.Namespace).Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "reader", OwnerReferences: []metav1.OwnerReference{{Kind: "VirtualMachineExport", Name: name, UID: "export-source"}}}}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	status, err = c.release(ctx, name)
	if err != nil || status.Phase != "releasing" {
		t.Fatal("released while exporter still existed", status, err)
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil || current.State.Admission == nil {
		t.Fatal("VM was unprotected while export reader existed", err)
	}
	if err = f.core.CoreV1().Pods(f.scope.Namespace).Delete(ctx, "reader", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	status, err = c.release(ctx, name)
	if err != nil || status.Phase != "released" {
		t.Fatal(status, err)
	}
	if _, err = c.release(ctx, name); err != nil {
		t.Fatal("release did not replay", err)
	}
}

// capability_id: rainbond.vm-delivery.current-system-disk
func TestVMDeliveryRejectsReplacedPVCAndCanCleanRejectedAdmission(t *testing.T) {
	c, f, name := deliveryCaptureFixture(t)
	ctx := context.Background()
	if _, err := c.prepare(ctx, name); err != nil {
		t.Fatal(err)
	}
	pvc, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "active-a", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pvc.UID = "replacement"
	if _, err = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, pvc, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.prepare(ctx, name); err == nil {
		t.Fatal("same-name replacement inherited source identity")
	}
	if _, err := c.release(ctx, name); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ClaimDeliveryCapture(f.scope, "foreign-capture"); err != nil {
		t.Fatal(err)
	}
	status, err := c.release(ctx, "delivery-"+strings.Repeat("b", 32))
	if err != nil || status.Phase != "released" {
		t.Fatal("never-admitted job could not settle", err)
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil || current.State.Admission == nil || current.State.Admission.Token != "foreign-capture" {
		t.Fatal("foreign admission was released", err)
	}
}

// capability_id: rainbond.vm-delivery.current-system-disk
func TestVMDeliveryFacadeRejectsForeignTenantAndReturnsOnlyOwnedProgress(t *testing.T) {
	_, f, name := deliveryCaptureFixture(t)
	previousManager, previousDynamic := defaultServieHandler, vmExportDynamicClient
	t.Cleanup(func() { defaultServieHandler = previousManager; vmExportDynamicClient = previousDynamic })
	f.action.dbmanager = deliveryCaptureManager{bindingConsumerDB: bindingConsumerDB{database: f.database}, tenant: &dbmodel.Tenants{UUID: f.scope.TenantID, Namespace: f.scope.Namespace}}
	defaultServieHandler = f.action
	vmExportDynamicClient = func() dynamic.Interface { return f.dynamic }
	ctx := context.WithValue(context.Background(), ctxutil.ContextKey("tenant_id"), "foreign")
	if _, err := VMDeliveryCaptureCommand(ctx, f.scope.ServiceID, name, "prepare", VMDeliveryRequest{}); err == nil {
		t.Fatal("foreign tenant admitted")
	}
	ctx = context.WithValue(context.Background(), ctxutil.ContextKey("tenant_id"), f.scope.TenantID)
	for _, action := range []string{"prepare", "status", "release"} {
		result, err := VMDeliveryCaptureCommand(ctx, f.scope.ServiceID, name, action, VMDeliveryRequest{})
		if err != nil || result.CaptureID != name {
			t.Fatal(action, result, err)
		}
	}
	if _, err := VMDeliveryCaptureCommand(ctx, f.scope.ServiceID, "invalid", "prepare", VMDeliveryRequest{}); err == nil {
		t.Fatal("invalid capture admitted")
	}
}

// capability_id: rainbond.vm-delivery.current-system-disk
func TestVMDeliveryRejectsExternalDiskReaders(t *testing.T) {
	c, f, name := deliveryCaptureFixture(t)
	ctx := context.Background()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "external-writer"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "shared", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "active-a"}}}}}}
	if _, err := f.core.CoreV1().Pods(f.scope.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.prepare(ctx, name); err != ErrVMDeliveryDiskInUse {
		t.Fatal("disk with another writer was captured", err)
	}
	if _, err := c.release(ctx, name); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-delivery.current-system-disk
func TestVMDeliveryInvalidSourcesNeverPublish(t *testing.T) {
	for _, scenario := range []string{"running", "pending-disk", "foreign-disk", "missing-vm", "missing-pvc", "missing-uid"} {
		t.Run(scenario, func(t *testing.T) {
			c, f, name := deliveryCaptureFixture(t)
			ctx := context.Background()
			vm, err := f.dynamic.Resource(deliveryVMGVR).Namespace(f.scope.Namespace).Get(ctx, "source", metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			pvc, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "active-a", metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "running":
				unstructured.SetNestedField(vm.Object, true, "spec", "running")
				_, err = f.dynamic.Resource(deliveryVMGVR).Namespace(f.scope.Namespace).Update(ctx, vm, metav1.UpdateOptions{})
			case "missing-uid":
				vm.SetUID("")
				_, err = f.dynamic.Resource(deliveryVMGVR).Namespace(f.scope.Namespace).Update(ctx, vm, metav1.UpdateOptions{})
			case "pending-disk":
				pvc.Status.Phase = corev1.ClaimPending
				_, err = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, pvc, metav1.UpdateOptions{})
			case "foreign-disk":
				pvc.Labels["service_id"] = "foreign"
				_, err = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, pvc, metav1.UpdateOptions{})
			case "missing-vm":
				err = f.dynamic.Resource(deliveryVMGVR).Namespace(f.scope.Namespace).Delete(ctx, "source", metav1.DeleteOptions{})
			case "missing-pvc":
				err = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Delete(ctx, "active-a", metav1.DeleteOptions{})
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.prepare(ctx, name); err == nil {
				t.Fatal("invalid source accepted")
			}
			if len(f.mq.tasks) != 0 {
				t.Fatal("invalid source started an image build")
			}
			if _, err := c.release(ctx, name); err != nil {
				t.Fatal("rejected source could not be cleaned", err)
			}
		})
	}
}

// capability_id: rainbond.vm-delivery.current-system-disk
func TestVMDeliveryBuildRequiresReadyCompressedExport(t *testing.T) {
	c, f, name := deliveryCaptureFixture(t)
	ctx := context.Background()
	if _, err := c.prepare(ctx, name); err != nil {
		t.Fatal(err)
	}
	request := VMDeliveryRequest{Arch: "amd64"}
	// An unconfigured delivery registry must use the Region's local registry.
	if _, err := c.image(ctx, name, request); err == nil {
		t.Fatal("unready disk started a build")
	}
	export, err := f.dynamic.Resource(vmExportGVR).Namespace(f.scope.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	export.Object["status"] = map[string]interface{}{"phase": "Ready", "links": map[string]interface{}{"internal": map[string]interface{}{"volumes": []interface{}{map[string]interface{}{"formats": []interface{}{map[string]interface{}{"format": "raw", "url": "https://virt-export-source.team.svc/volumes/root/disk.img"}}}}}}}
	if _, err = f.dynamic.Resource(vmExportGVR).Namespace(f.scope.Namespace).Update(ctx, export, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.image(ctx, name, request); err == nil {
		t.Fatal("uncompressed export accepted")
	}
	export.Object["status"] = map[string]interface{}{"phase": "Ready", "links": map[string]interface{}{"internal": map[string]interface{}{"volumes": []interface{}{map[string]interface{}{"formats": []interface{}{map[string]interface{}{"format": "gzip", "url": "https://virt-export-source.team.svc/volumes/root/disk.img.gz"}}}}}}}
	if _, err = f.dynamic.Resource(vmExportGVR).Namespace(f.scope.Namespace).Update(ctx, export, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.image(ctx, name, request); err != nil {
		t.Fatal(err)
	}
	c.shareStatus = func(string) (string, error) { return "failure", nil }
	result, err := c.status(ctx, name)
	if err != nil || result.Phase != "failed" {
		t.Fatal("failed builder became successful", result, err)
	}
	if _, err := c.release(ctx, name); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-delivery.current-system-disk
func TestVMDeliveryRejectsAlteredExportOwnership(t *testing.T) {
	c, f, name := deliveryCaptureFixture(t)
	ctx := context.Background()
	if _, err := c.prepare(ctx, name); err != nil {
		t.Fatal(err)
	}
	resource := f.dynamic.Resource(vmExportGVR).Namespace(f.scope.Namespace)
	export, err := resource.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	labels := export.GetLabels()
	labels["service_id"] = "foreign"
	export.SetLabels(labels)
	if _, err = resource.Update(ctx, export, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.status(ctx, name); err == nil {
		t.Fatal("foreign export exposed through status")
	}
	if _, err := c.release(ctx, name); err == nil {
		t.Fatal("foreign export deleted")
	}
	labels["service_id"] = f.scope.ServiceID
	export.SetLabels(labels)
	export.Object["status"] = map[string]interface{}{"phase": "Ready", "links": map[string]interface{}{"internal": map[string]interface{}{"volumes": []interface{}{map[string]interface{}{"formats": []interface{}{map[string]interface{}{"format": "gzip", "url": "https://virt-export-source.team.svc/volumes/root/disk.img.gz"}}}}}}}
	unstructured.SetNestedField(export.Object, "foreign-pvc", "spec", "source", "name")
	if _, err = resource.Update(ctx, export, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	request := VMDeliveryRequest{Arch: "amd64"}
	request.ImageInfo.HubURL = "registry.test"
	if _, err := c.image(ctx, name, request); err == nil {
		t.Fatal("changed export source was published")
	}
	if len(f.mq.tasks) != 0 {
		t.Fatal("foreign disk was queued")
	}
	if _, err := c.release(ctx, name); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-delivery.current-system-disk
func TestVMDeliveryWaitsForExportDeletionBeforeReleasingAdmission(t *testing.T) {
	c, f, name := deliveryCaptureFixture(t)
	ctx := context.Background()
	if _, err := c.prepare(ctx, name); err != nil {
		t.Fatal(err)
	}
	f.dynamic.PrependReactor("delete", "virtualmachineexports", func(k8stesting.Action) (bool, runtime.Object, error) { return true, nil, nil })
	result, err := c.release(ctx, name)
	if err != nil || result.Phase != "releasing" {
		t.Fatal("pending export deletion treated as released", result, err)
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil || current.State.Admission == nil {
		t.Fatal("export still exists but admission cleared", err)
	}
}
