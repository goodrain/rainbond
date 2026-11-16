package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/goodrain/rainbond/builder"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	kv "kubevirt.io/api/core/v1"
)

const deliveryCaptureLabel = "rainbond.com/delivery-capture"
const deliveryCapturePrefix = "rainbond.com/delivery-"

var deliveryCaptureName = regexp.MustCompile(`^delivery-[a-f0-9]{32}$`)

// ErrVMDeliveryShutdown is safe to expose without upstream error details.
var ErrVMDeliveryShutdown = errors.New("vm_delivery_shutdown_required")

// ErrVMDeliveryDiskCount prevents silently dropping additional writable disks.
var ErrVMDeliveryDiskCount = errors.New("vm_delivery_single_disk_required")

// ErrVMDeliveryDiskInUse rejects a disk with readers outside this capture.
var ErrVMDeliveryDiskInUse = errors.New("vm_delivery_disk_in_use")
var deliveryVMGVR = kv.SchemeGroupVersion.WithResource("virtualmachines")
var deliveryVMIGVR = kv.SchemeGroupVersion.WithResource("virtualmachineinstances")

// VMDeliveryRequest accepts destination credentials only for enqueueing. They
// are never copied to status, persistent task metadata or Kubernetes annotations.
type VMDeliveryRequest struct {
	Arch      string `json:"arch"`
	ImageInfo struct {
		HubURL      string `json:"hub_url"`
		HubUser     string `json:"hub_user"`
		HubPassword string `json:"hub_password"`
		Namespace   string `json:"namespace"`
	} `json:"image_info"`
}

// VMDeliveryStatus contains only public progress and immutable artifact identity.
type VMDeliveryStatus struct {
	CaptureID   string `json:"capture_id"`
	Phase       string `json:"phase"`
	ImageName   string `json:"image_name,omitempty"`
	ShareID     string `json:"share_id,omitempty"`
	EventID     string `json:"event_id,omitempty"`
	BootMode    string `json:"boot_mode,omitempty"`
	RequestSize string `json:"request_size,omitempty"`
	CapacityGiB int64  `json:"capacity_gib,omitempty"`
}

type vmDeliveryCapture struct {
	scope       vmrestore.Scope
	store       *vmrestore.Store
	core        kubernetes.Interface
	dynamic     dynamic.Interface
	mq          client.MQClient
	shareStatus func(string) (string, error)
}

// VMDeliveryCaptureCommand uses the same authenticated scope and persistent
// admission as existing VM power/storage operations, without changing their API.
func VMDeliveryCaptureCommand(ctx context.Context, serviceID, name, action string, request VMDeliveryRequest) (*VMDeliveryStatus, error) {
	service, ok := GetServiceManager().(*ServiceAction)
	if !ok || !deliveryCaptureName.MatchString(name) {
		return nil, fmt.Errorf("invalid VM delivery capture")
	}
	scope, err := service.snapshotScope(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	manager := service.getDBManager()
	if manager.DB() == nil || service.kubeClient == nil || vmExportDynamicClient() == nil {
		return nil, vmrestore.ErrReferenceUnavailable
	}
	capture := &vmDeliveryCapture{scope: scope, store: vmrestore.NewStore(manager.DB()), core: service.kubeClient, dynamic: vmExportDynamicClient(), mq: service.MQClient,
		shareStatus: func(id string) (string, error) {
			row, err := db.GetManager().KeyValueDao().Get("/rainbond/shareresult/" + id)
			if err != nil || row == nil {
				return "", err
			}
			var result struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal([]byte(row.V), &result); err != nil {
				return "", err
			}
			return result.Status, nil
		}}
	switch action {
	case "prepare":
		return capture.prepare(ctx, name)
	case "status":
		return capture.status(ctx, name)
	case "image":
		return capture.image(ctx, name, request)
	case "release":
		return capture.release(ctx, name)
	default:
		return nil, fmt.Errorf("invalid VM delivery action")
	}
}

// Installed system disks must never be confused with ISO installation media.
// Until multi-disk capture is implemented, rejecting another writable disk is
// safer than publishing an apparently complete VM with missing application data.
func resolveVMDeliveryRootPVC(vm *kv.VirtualMachine) (string, error) {
	if vm == nil || vm.Spec.Template == nil {
		return "", fmt.Errorf("VM template is unavailable")
	}
	var names []string
	for _, disk := range vm.Spec.Template.Spec.Domain.Devices.Disks {
		if disk.CDRom != nil {
			continue
		}
		for _, volume := range vm.Spec.Template.Spec.Volumes {
			if disk.Name != volume.Name {
				continue
			}
			if volume.DataVolume != nil {
				names = append(names, volume.DataVolume.Name)
			}
			if volume.PersistentVolumeClaim != nil && !volume.PersistentVolumeClaim.ReadOnly {
				names = append(names, volume.PersistentVolumeClaim.ClaimName)
			}
		}
	}
	if len(names) != 1 || names[0] == "" {
		return "", ErrVMDeliveryDiskCount
	}
	return names[0], nil
}

func (c *vmDeliveryCapture) owned(obj metav1.Object, name string) bool {
	return obj.GetLabels()["service_id"] == c.scope.ServiceID && obj.GetLabels()[deliveryCaptureLabel] == name
}
func (c *vmDeliveryCapture) admission(name string) error {
	current, err := c.store.Read(c.scope, "")
	if err != nil {
		return err
	}
	if current.State.Fence != nil || current.State.Admission == nil || current.State.Admission.Token != name || current.State.Admission.Action != "delivery-capture" {
		return vmrestore.ErrConflict
	}
	return nil
}

func (c *vmDeliveryCapture) stoppedVM(ctx context.Context) (*kv.VirtualMachine, string, error) {
	vms, err := c.dynamic.Resource(deliveryVMGVR).Namespace(c.scope.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "service_id=" + c.scope.ServiceID})
	if err != nil {
		return nil, "", err
	}
	if len(vms.Items) != 1 {
		return nil, "", fmt.Errorf("exactly one source VM is required")
	}
	var vm kv.VirtualMachine
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(vms.Items[0].Object, &vm); err != nil {
		return nil, "", err
	}
	if vm.UID == "" || vm.DeletionTimestamp != nil {
		return nil, "", vmrestore.ErrConflict
	}
	strategy, err := vm.RunStrategy()
	if err != nil || (strategy != kv.RunStrategyHalted && strategy != kv.RunStrategyManual) {
		return nil, "", ErrVMDeliveryShutdown
	}
	if _, err := c.dynamic.Resource(deliveryVMIGVR).Namespace(c.scope.Namespace).Get(ctx, vm.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		if err != nil {
			return nil, "", err
		}
		return nil, "", ErrVMDeliveryShutdown
	}
	pvc, err := resolveVMDeliveryRootPVC(&vm)
	return &vm, pvc, err
}

func (c *vmDeliveryCapture) prepare(ctx context.Context, name string) (*VMDeliveryStatus, error) {
	if !deliveryCaptureName.MatchString(name) {
		return nil, vmrestore.ErrCorrupt
	}
	if err := c.store.ClaimDeliveryCapture(c.scope, name); err != nil {
		return nil, err
	}
	// All failures remain owned until release has verified readers are gone.
	vm, pvcName, err := c.stoppedVM(ctx)
	if err != nil {
		return nil, err
	}
	pvc, err := c.core.CoreV1().PersistentVolumeClaims(c.scope.Namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if pvc.UID == "" || pvc.DeletionTimestamp != nil || pvc.Status.Phase != corev1.ClaimBound {
		return nil, vmrestore.ErrConflict
	}
	exports := c.dynamic.Resource(vmExportGVR).Namespace(c.scope.Namespace)
	existing, err := exports.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		if !c.owned(existing, name) || existing.GetAnnotations()[deliveryCapturePrefix+"vm-uid"] != string(vm.UID) || existing.GetAnnotations()[deliveryCapturePrefix+"pvc-uid"] != string(pvc.UID) {
			return nil, vmrestore.ErrConflict
		}
		return c.status(ctx, name)
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}
	if pvc.Labels["service_id"] != c.scope.ServiceID {
		return nil, vmrestore.ErrConflict
	}
	if err := c.readers(ctx, pvcName, ""); err != nil {
		return nil, err
	}
	secrets := c.core.CoreV1().Secrets(c.scope.Namespace)
	secret, err := secrets.Get(ctx, name+"-token", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		secret, err = secrets.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name + "-token", Namespace: c.scope.Namespace, Labels: map[string]string{"service_id": c.scope.ServiceID, deliveryCaptureLabel: name}}, Data: map[string][]byte{"token": []byte(uuid.NewString())}}, metav1.CreateOptions{})
	}
	if err != nil {
		return nil, err
	}
	if !c.owned(secret, name) || secret.Annotations[deliveryCapturePrefix+"export-uid"] != "" {
		return nil, vmrestore.ErrConflict
	}
	export := buildVMExport(c.scope.Namespace, name, "VM delivery system disk", c.scope.ServiceID, pvcName, secret.Name)
	export.SetLabels(map[string]string{"service_id": c.scope.ServiceID, deliveryCaptureLabel: name})
	boot := "bios"
	if vm.Spec.Template.Spec.Domain.Firmware != nil && vm.Spec.Template.Spec.Domain.Firmware.Bootloader != nil && vm.Spec.Template.Spec.Domain.Firmware.Bootloader.EFI != nil {
		boot = "uefi"
	}
	export.SetAnnotations(map[string]string{deliveryCapturePrefix + "vm-uid": string(vm.UID), deliveryCapturePrefix + "pvc-uid": string(pvc.UID), deliveryCapturePrefix + "boot-mode": boot})
	a := export.GetAnnotations()
	size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if capacity := pvc.Status.Capacity[corev1.ResourceStorage]; capacity.Cmp(size) > 0 {
		size = capacity
	}
	a[deliveryCapturePrefix+"disk-size"] = size.String()
	a[deliveryCapturePrefix+"disk-gib"] = fmt.Sprint((size.Value() + (1 << 30) - 1) / (1 << 30))
	export.SetAnnotations(a)
	export, err = exports.Create(ctx, export, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return c.prepare(ctx, name)
	}
	if err != nil {
		return nil, err
	}
	secret.Annotations = map[string]string{deliveryCapturePrefix + "export-uid": string(export.GetUID())}
	if _, err = secrets.Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		return nil, err
	}
	return c.status(ctx, name)
}

func (c *vmDeliveryCapture) status(ctx context.Context, name string) (*VMDeliveryStatus, error) {
	if err := c.admission(name); err != nil {
		return nil, err
	}
	export, err := c.dynamic.Resource(vmExportGVR).Namespace(c.scope.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !c.owned(export, name) {
		return nil, vmrestore.ErrConflict
	}
	a := export.GetAnnotations()
	result := &VMDeliveryStatus{CaptureID: name, Phase: "exporting", BootMode: a[deliveryCapturePrefix+"boot-mode"], ShareID: a[deliveryCapturePrefix+"share-id"], EventID: a[deliveryCapturePrefix+"event-id"], ImageName: a[deliveryCapturePrefix+"image"]}
	result.RequestSize = a[deliveryCapturePrefix+"disk-size"]
	fmt.Sscan(a[deliveryCapturePrefix+"disk-gib"], &result.CapacityGiB)
	if result.ImageName == "" {
		result.ImageName = strings.TrimSuffix(builder.REGISTRYDOMAIN, "/") + "/" + c.scope.ServiceID + ":" + name
	}
	if result.ShareID != "" {
		result.Phase = "building"
		state, err := c.shareStatus(result.ShareID)
		if err != nil {
			return nil, err
		}
		if state == "success" {
			result.Phase = "success"
		}
		if state == "failure" || state == "failed" || state == "timeout" {
			result.Phase = "failed"
		}
	} else if nestedString(export.Object, "status", "phase") == "Ready" && extractVMExportDownloadURL(export.Object) != "" {
		result.Phase = "ready"
	}
	return result, nil
}

func (c *vmDeliveryCapture) image(ctx context.Context, name string, request VMDeliveryRequest) (*VMDeliveryStatus, error) {
	status, err := c.status(ctx, name)
	if err != nil {
		return nil, err
	}
	if status.ShareID != "" {
		return status, nil
	}
	if status.Phase != "ready" || (request.Arch != "amd64" && request.Arch != "arm64") {
		return nil, fmt.Errorf("VM export or image destination is not ready")
	}
	vm, pvcName, err := c.stoppedVM(ctx)
	if err != nil {
		return nil, err
	}
	exports := c.dynamic.Resource(vmExportGVR).Namespace(c.scope.Namespace)
	export, err := exports.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	annotations := export.GetAnnotations()
	pvc, err := c.core.CoreV1().PersistentVolumeClaims(c.scope.Namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !c.owned(export, name) || annotations[deliveryCapturePrefix+"vm-uid"] != string(vm.UID) || annotations[deliveryCapturePrefix+"pvc-uid"] != string(pvc.UID) {
		return nil, vmrestore.ErrConflict
	}
	if nestedString(export.Object, "spec", "source", "name") != pvcName || nestedString(export.Object, "spec", "source", "kind") != "PersistentVolumeClaim" || nestedString(export.Object, "spec", "tokenSecretRef") != name+"-token" {
		return nil, vmrestore.ErrConflict
	}
	if err := c.readers(ctx, pvcName, string(export.GetUID())); err != nil {
		return nil, err
	}
	source := extractVMExportDownloadURL(export.Object)
	if !strings.HasSuffix(strings.Split(source, "?")[0], "disk.img.gz") {
		return nil, fmt.Errorf("compressed system disk export is required")
	}
	token, err := readVMExportDownloadToken(c.core, c.scope.Namespace, name+"-token")
	if err != nil {
		return nil, err
	}
	shareID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(c.scope.TenantID+"/"+c.scope.ServiceID+"/"+name)).String()
	hub := request.ImageInfo.HubURL
	if hub == "" {
		hub = builder.REGISTRYDOMAIN
	}
	image := strings.TrimSuffix(hub, "/") + "/"
	if request.ImageInfo.Namespace != "" {
		image += strings.Trim(request.ImageInfo.Namespace, "/") + "/"
	}
	image += c.scope.ServiceID + ":" + name
	annotations[deliveryCapturePrefix+"share-id"] = shareID
	annotations[deliveryCapturePrefix+"event-id"] = strings.ReplaceAll(shareID, "-", "")
	annotations[deliveryCapturePrefix+"image"] = image
	export.SetAnnotations(annotations)
	// ResourceVersion is the enqueue claim: ambiguous sends are never repeated.
	if _, err = exports.Update(ctx, export, metav1.UpdateOptions{}); err != nil {
		return nil, err
	}
	info := map[string]interface{}{"hub_url": request.ImageInfo.HubURL, "hub_user": request.ImageInfo.HubUser, "hub_password": request.ImageInfo.HubPassword, "namespace": request.ImageInfo.Namespace, "vm_image_source": source, "vm_image_token": token, "vm_capture_id": name}
	body := map[string]interface{}{"service_id": c.scope.ServiceID, "tenant_name": c.scope.Namespace, "arch": request.Arch, "image_name": image, "share_id": shareID, "share_info": map[string]interface{}{"service_key": c.scope.ServiceID, "app_version": name, "event_id": annotations[deliveryCapturePrefix+"event-id"], "share_scope": "delivery", "image_info": info}}
	if err := c.mq.SendBuilderTopic(client.TaskStruct{Topic: client.BuilderTopic, Arch: request.Arch, TaskType: "share-image", TaskBody: body}); err != nil {
		return nil, err
	}
	return c.status(ctx, name)
}

func (c *vmDeliveryCapture) readers(ctx context.Context, pvcName, exportUID string) error {
	pods, err := c.core.CoreV1().Pods(c.scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		owned := false
		for _, owner := range pod.OwnerReferences {
			if exportUID != "" && string(owner.UID) == exportUID {
				owned = true
			}
		}
		if owned {
			continue
		}
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
				return ErrVMDeliveryDiskInUse
			}
		}
	}
	return nil
}

// Release first removes the export, then observes exporter Pods gone. A queued
// or running builder can no longer read a changing source after admission clears.
func (c *vmDeliveryCapture) release(ctx context.Context, name string) (*VMDeliveryStatus, error) {
	if !deliveryCaptureName.MatchString(name) {
		return nil, vmrestore.ErrCorrupt
	}
	result := &VMDeliveryStatus{CaptureID: name, Phase: "releasing"}
	current, err := c.store.Read(c.scope, "")
	if err != nil {
		return nil, err
	}
	if current.State.Admission == nil {
		result.Phase = "released"
		return result, nil
	}
	if current.State.Admission.Token != name {
		// A preparation rejected by another VM operation owns no resources.
		_, exportErr := c.dynamic.Resource(vmExportGVR).Namespace(c.scope.Namespace).Get(ctx, name, metav1.GetOptions{})
		_, secretErr := c.core.CoreV1().Secrets(c.scope.Namespace).Get(ctx, name+"-token", metav1.GetOptions{})
		if apierrors.IsNotFound(exportErr) && apierrors.IsNotFound(secretErr) {
			result.Phase = "released"
			return result, nil
		}
	}
	if err := c.admission(name); err != nil {
		return nil, err
	}
	exports := c.dynamic.Resource(vmExportGVR).Namespace(c.scope.Namespace)
	export, err := exports.Get(ctx, name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	uid := ""
	if err == nil {
		if !c.owned(export, name) || export.GetUID() == "" {
			return nil, vmrestore.ErrConflict
		}
		uid = string(export.GetUID())
	}
	secrets := c.core.CoreV1().Secrets(c.scope.Namespace)
	secret, secretErr := secrets.Get(ctx, name+"-token", metav1.GetOptions{})
	if secretErr != nil && !apierrors.IsNotFound(secretErr) {
		return nil, secretErr
	}
	if secretErr == nil {
		if !c.owned(secret, name) {
			return nil, vmrestore.ErrConflict
		}
		if uid == "" {
			uid = secret.Annotations[deliveryCapturePrefix+"export-uid"]
		} else {
			if secret.Annotations == nil {
				secret.Annotations = map[string]string{}
			}
			secret.Annotations[deliveryCapturePrefix+"export-uid"] = uid
			if _, err := secrets.Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
				return nil, err
			}
		}
	}
	if export != nil {
		objectUID := types.UID(uid)
		if err := exports.Delete(ctx, name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &objectUID}}); err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
		if _, err := exports.Get(ctx, name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
			return result, err
		}
	}
	pods, err := c.core.CoreV1().Pods(c.scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		for _, owner := range pod.OwnerReferences {
			if (uid != "" && string(owner.UID) == uid) || (owner.Kind == "VirtualMachineExport" && owner.Name == name) {
				return result, nil
			}
		}
	}
	if secretErr == nil {
		if err := secrets.Delete(ctx, secret.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &secret.UID}}); err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	if err := c.store.ReleaseDeliveryCapture(c.scope, name); err != nil {
		return nil, err
	}
	result.Phase = "released"
	return result, nil
}
