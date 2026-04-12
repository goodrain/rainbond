package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/pkg/component/k8s"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

var (
	vmDataExportGVR = schema.GroupVersionResource{
		Group:    "cdi.kubevirt.io",
		Version:  "v1beta1",
		Resource: "dataexports",
	}
	vmExportDynamicClient = func() dynamic.Interface {
		return k8s.Default().DynamicClient
	}
)

type VMExportRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	ExportAllDisks bool   `json:"export_all_disks"`
	SourceKind     string `json:"source_kind,omitempty"`
	SnapshotName   string `json:"snapshot_name,omitempty"`
}

type VMExportDisk struct {
	DiskKey      string `json:"disk_key"`
	DiskName     string `json:"disk_name"`
	DiskRole     string `json:"disk_role"`
	BootOrder    uint   `json:"boot_order,omitempty"`
	PVCName      string `json:"pvc_name"`
	PVCNamespace string `json:"pvc_namespace"`
	ExportName   string `json:"export_name,omitempty"`
	Status       string `json:"status,omitempty"`
	DownloadURL  string `json:"download_url,omitempty"`
	Message      string `json:"message,omitempty"`
}

type VMExportStatus struct {
	ExportID string         `json:"export_id"`
	Name     string         `json:"name,omitempty"`
	Status   string         `json:"status"`
	Message  string         `json:"message,omitempty"`
	Disks    []VMExportDisk `json:"disks"`
}

func (s *ServiceAction) StartVMExport(serviceID, exportID string, req *VMExportRequest) (*VMExportStatus, error) {
	if vmExportRequiresClosedVM(req) {
		vmis, err := s.kubevirtClient.VirtualMachineInstance("").List(context.Background(), metav1.ListOptions{
			LabelSelector: "service_id=" + serviceID,
		})
		if err != nil {
			return nil, err
		}
		if len(vmis.Items) > 0 {
			return nil, ErrServiceNotClosed
		}
	}

	vms, err := s.kubevirtClient.VirtualMachine("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "service_id=" + serviceID,
	})
	if err != nil {
		return nil, err
	}
	if len(vms.Items) == 0 {
		return nil, fmt.Errorf("service id is %v vm is not exist", serviceID)
	}

	vm := &vms.Items[0]
	disks := discoverVMExportDisks(vm)
	if len(disks) == 0 {
		return nil, fmt.Errorf("service id is %v vm has no persistent disks for export", serviceID)
	}
	if !hasPersistentRootDisk(disks) {
		return nil, fmt.Errorf("service id is %v vm has no persistent root disk for export", serviceID)
	}
	if err := createVMDataExports(vmExportDynamicClient(), exportID, serviceID, vm, disks); err != nil {
		return nil, err
	}
	for i := range disks {
		disks[i].ExportName = buildVMExportName(exportID, disks[i].DiskKey)
		disks[i].Status = "exporting"
	}
	return &VMExportStatus{
		ExportID: exportID,
		Name:     req.Name,
		Status:   "exporting",
		Disks:    disks,
	}, nil
}

func (s *ServiceAction) GetVMExportStatus(serviceID, exportID string) (*VMExportStatus, error) {
	return BuildVMExportStatus(vmExportDynamicClient(), serviceID, exportID)
}

func BuildVMExportStatus(dynamicClient dynamic.Interface, serviceID, exportID string) (*VMExportStatus, error) {
	if dynamicClient == nil {
		return nil, fmt.Errorf("dynamic client is nil")
	}
	list, err := dynamicClient.Resource(vmDataExportGVR).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	disks := make([]VMExportDisk, 0)
	for _, item := range list.Items {
		labels := item.GetLabels()
		if labels["service_id"] != serviceID || labels["vm_export_id"] != exportID {
			continue
		}
		disk := VMExportDisk{
			DiskKey:      labels["vm_export_disk_key"],
			DiskRole:     labels["vm_export_disk_role"],
			ExportName:   item.GetName(),
			DiskName:     item.GetAnnotations()["vm_export_disk_name"],
			BootOrder:    parseVMExportUint(item.GetAnnotations()["vm_export_boot_order"]),
			Status:       normalizeVMExportPhase(getNestedString(item.Object, "status", "phase")),
			PVCName:      getNestedString(item.Object, "spec", "source", "pvc", "name"),
			PVCNamespace: getNestedString(item.Object, "spec", "source", "pvc", "namespace"),
			DownloadURL:  extractVMExportURL(item.Object),
			Message:      extractVMExportMessage(item.Object),
		}
		if disk.DiskKey == "" {
			disk.DiskKey = item.GetName()
		}
		if disk.DiskName == "" {
			disk.DiskName = disk.DiskKey
		}
		if disk.DiskRole == "" {
			disk.DiskRole = "data"
		}
		disks = append(disks, disk)
	}
	if len(disks) == 0 {
		return nil, fmt.Errorf("vm export %s not found", exportID)
	}
	sort.Slice(disks, func(i, j int) bool {
		iRank := vmExportDiskRoleRank(disks[i].DiskRole)
		jRank := vmExportDiskRoleRank(disks[j].DiskRole)
		if iRank == jRank {
			if disks[i].BootOrder != 0 && disks[j].BootOrder != 0 && disks[i].BootOrder != disks[j].BootOrder {
				return disks[i].BootOrder < disks[j].BootOrder
			}
			return disks[i].DiskKey < disks[j].DiskKey
		}
		return iRank < jRank
	})
	return &VMExportStatus{
		ExportID: exportID,
		Status:   aggregateVMExportStatus(disks),
		Disks:    disks,
	}, nil
}

func discoverVMExportDisks(vm *kubevirtv1.VirtualMachine) []VMExportDisk {
	if vm == nil || vm.Spec.Template == nil {
		return nil
	}
	bootOrderMap := make(map[string]uint)
	for _, disk := range vm.Spec.Template.Spec.Domain.Devices.Disks {
		if disk.BootOrder != nil {
			bootOrderMap[disk.Name] = *disk.BootOrder
		}
	}
	disks := make([]VMExportDisk, 0)
	for _, volume := range vm.Spec.Template.Spec.Volumes {
		pvcName := vmExportPVCName(volume)
		if pvcName == "" {
			continue
		}
		disks = append(disks, VMExportDisk{
			DiskKey:      volume.Name,
			DiskName:     volume.Name,
			DiskRole:     "data",
			BootOrder:    bootOrderMap[volume.Name],
			PVCName:      pvcName,
			PVCNamespace: vm.Namespace,
		})
	}
	sort.Slice(disks, func(i, j int) bool {
		iOrder, iOK := bootOrderMap[disks[i].DiskKey]
		jOrder, jOK := bootOrderMap[disks[j].DiskKey]
		if iOK && jOK && iOrder != jOrder {
			return iOrder < jOrder
		}
		if iOK != jOK {
			return iOK
		}
		return disks[i].DiskKey < disks[j].DiskKey
	})
	if len(disks) > 0 {
		for _, disk := range vm.Spec.Template.Spec.Domain.Devices.Disks {
			if disk.BootOrder != nil && *disk.BootOrder == 1 {
				for i := range disks {
					if disks[i].DiskKey == disk.Name {
						disks[i].DiskRole = "root"
						return disks
					}
				}
				break
			}
		}
	}
	return disks
}

func vmExportPVCName(volume kubevirtv1.Volume) string {
	if volume.PersistentVolumeClaim != nil {
		return volume.PersistentVolumeClaim.ClaimName
	}
	if volume.DataVolume != nil {
		return strings.TrimSpace(volume.DataVolume.Name)
	}
	return ""
}

func hasPersistentRootDisk(disks []VMExportDisk) bool {
	for _, disk := range disks {
		if disk.DiskRole == "root" {
			return true
		}
	}
	return false
}

func createVMDataExports(dynamicClient dynamic.Interface, exportID, serviceID string, vm *kubevirtv1.VirtualMachine, disks []VMExportDisk) error {
	if dynamicClient == nil {
		return fmt.Errorf("dynamic client is nil")
	}
	for _, disk := range disks {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cdi.kubevirt.io/v1beta1",
				"kind":       "DataExport",
				"metadata": map[string]interface{}{
					"name":      buildVMExportName(exportID, disk.DiskKey),
					"namespace": vm.Namespace,
					"labels": map[string]interface{}{
						"service_id":          serviceID,
						"vm_export_id":        exportID,
						"vm_export_disk_key":  disk.DiskKey,
						"vm_export_disk_role": disk.DiskRole,
					},
					"annotations": map[string]interface{}{
						"vm_export_disk_name":  disk.DiskName,
						"vm_export_boot_order": fmt.Sprintf("%d", disk.BootOrder),
					},
				},
				"spec": map[string]interface{}{
					"source": map[string]interface{}{
						"pvc": map[string]interface{}{
							"name":      disk.PVCName,
							"namespace": disk.PVCNamespace,
						},
					},
				},
			},
		}
		_, err := dynamicClient.Resource(vmDataExportGVR).Namespace(vm.Namespace).Create(context.Background(), obj, metav1.CreateOptions{})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func buildVMExportName(exportID, diskKey string) string {
	return fmt.Sprintf("%s-%s", sanitizeVMExportName(exportID), sanitizeVMExportName(diskKey))
}

func sanitizeVMExportName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, ".", "-")
	if value == "" {
		return "vm-export"
	}
	return value
}

func normalizeVMExportPhase(phase string) string {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "ready", "succeeded", "complete":
		return "ready"
	case "failed", "error":
		return "failed"
	default:
		return "exporting"
	}
}

func aggregateVMExportStatus(disks []VMExportDisk) string {
	readyCount := 0
	for _, disk := range disks {
		switch disk.Status {
		case "failed":
			return "failed"
		case "ready":
			readyCount++
		}
	}
	if readyCount == len(disks) {
		return "ready"
	}
	return "exporting"
}

func vmExportDiskRoleRank(role string) int {
	switch role {
	case "root":
		return 0
	case "data":
		return 1
	default:
		return 2
	}
}

func extractVMExportURL(obj map[string]interface{}) string {
	urls, found, err := unstructured.NestedSlice(obj, "status", "links", "external", "urls")
	if err == nil && found {
		for _, value := range urls {
			item, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			if url, _, _ := unstructured.NestedString(item, "url"); url != "" {
				return url
			}
		}
	}
	urls, found, err = unstructured.NestedSlice(obj, "status", "links", "internal", "urls")
	if err == nil && found {
		for _, value := range urls {
			item, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			if url, _, _ := unstructured.NestedString(item, "url"); url != "" {
				return url
			}
		}
	}
	return ""
}

func getNestedString(obj map[string]interface{}, fields ...string) string {
	value, _, _ := unstructured.NestedString(obj, fields...)
	return value
}

func extractVMExportMessage(obj map[string]interface{}) string {
	conditions, found, err := unstructured.NestedSlice(obj, "status", "conditions")
	if err == nil && found {
		for _, value := range conditions {
			item, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			if message, _, _ := unstructured.NestedString(item, "message"); message != "" {
				return message
			}
		}
	}
	if message, _, _ := unstructured.NestedString(obj, "status", "message"); message != "" {
		return message
	}
	return ""
}

func marshalVMExportStatus(status *VMExportStatus) string {
	if status == nil {
		return ""
	}
	data, _ := json.Marshal(status)
	return string(data)
}

func vmExportRequiresClosedVM(req *VMExportRequest) bool {
	if req == nil {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(req.SourceKind), "snapshot") && strings.TrimSpace(req.SnapshotName) != "" {
		return false
	}
	return true
}

func parseVMExportUint(value string) uint {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 32)
	if err != nil {
		return 0
	}
	return uint(parsed)
}
