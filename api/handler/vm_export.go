package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

var (
	vmDataExportGVR = schema.GroupVersionResource{
		Group:    "export.kubevirt.io",
		Version:  "v1beta1",
		Resource: "virtualmachineexports",
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

type VMExportUploadedDisk struct {
	DiskKey    string `json:"disk_key"`
	ObjectKey  string `json:"object_key"`
	ObjectURI  string `json:"object_uri"`
	StorageURL string `json:"storage_url,omitempty"`
	Format     string `json:"format,omitempty"`
	SizeBytes  int64  `json:"size_bytes,omitempty"`
	Checksum   string `json:"checksum,omitempty"`
}

type VMMachineManifestDisk struct {
	DiskKey   string `json:"disk_key"`
	DiskName  string `json:"disk_name"`
	DiskRole  string `json:"disk_role"`
	BootOrder uint   `json:"boot_order,omitempty"`
	ObjectKey string `json:"object_key"`
	ObjectURI string `json:"object_uri,omitempty"`
	Format    string `json:"format,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Checksum  string `json:"checksum,omitempty"`
}

type VMMachineManifest struct {
	Version     string                  `json:"version"`
	Arch        string                  `json:"arch,omitempty"`
	BootMode    string                  `json:"boot_mode,omitempty"`
	RootDiskKey string                  `json:"root_disk_key"`
	Disks       []VMMachineManifestDisk `json:"disks"`
}

type VMExportPersistRequest struct {
	AssetID   int64  `json:"asset_id"`
	AssetName string `json:"asset_name,omitempty"`
}

type VMExportPersistStatus struct {
	ExportID        string             `json:"export_id"`
	Status          string             `json:"status"`
	StorageBackend  string             `json:"storage_backend,omitempty"`
	StorageBucket   string             `json:"storage_bucket,omitempty"`
	StoragePrefix   string             `json:"storage_prefix,omitempty"`
	RootObjectURI   string             `json:"root_object_uri,omitempty"`
	MachineManifest *VMMachineManifest `json:"machine_manifest,omitempty"`
}

type VMAssetRestorePlanRequest struct {
	Manifest *VMMachineManifest `json:"manifest"`
}

type VMAssetRestoreDiskImport struct {
	VolumeName string `json:"volume_name"`
	DiskKey    string `json:"disk_key"`
	DiskName   string `json:"disk_name"`
	ImageURL   string `json:"image_url"`
	SourceURI  string `json:"source_uri,omitempty"`
	Format     string `json:"format,omitempty"`
	Checksum   string `json:"checksum,omitempty"`
}

type VMAssetRestoreDiskLayoutItem struct {
	DiskKey    string `json:"disk_key"`
	DiskName   string `json:"disk_name"`
	DiskRole   string `json:"disk_role"`
	BootOrder  uint   `json:"boot_order,omitempty"`
	OrderIndex int    `json:"order_index,omitempty"`
	Boot       bool   `json:"boot"`
}

type VMAssetRestorePlan struct {
	BootSourceFormat string                         `json:"boot_source_format"`
	DiskImports      []VMAssetRestoreDiskImport     `json:"disk_imports"`
	DiskLayout       []VMAssetRestoreDiskLayoutItem `json:"disk_layout"`
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
		logrus.Infof(
			"vm export created: service_id=%s export_id=%s vm=%s disk_key=%s disk_role=%s pvc=%s/%s boot_order=%d",
			serviceID,
			exportID,
			vm.Name,
			disks[i].DiskKey,
			disks[i].DiskRole,
			disks[i].PVCNamespace,
			disks[i].PVCName,
			disks[i].BootOrder,
		)
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
			PVCName:      getNestedString(item.Object, "spec", "source", "name"),
			PVCNamespace: item.GetNamespace(),
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
		authSource, hasCert, hasToken := extractVMExportAuthSummary(item.Object)
		logrus.Infof(
			"vm export status: service_id=%s export_id=%s export_name=%s disk_key=%s disk_role=%s status=%s download_url=%s auth_source=%s cert=%t token=%t",
			serviceID,
			exportID,
			disk.ExportName,
			disk.DiskKey,
			disk.DiskRole,
			disk.Status,
			disk.DownloadURL,
			authSource,
			hasCert,
			hasToken,
		)
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
				"apiVersion": "export.kubevirt.io/v1beta1",
				"kind":       "VirtualMachineExport",
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
						"kind": "PersistentVolumeClaim",
						"name": disk.PVCName,
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
	for _, fields := range [][]string{
		{"status", "links", "external", "volumes"},
		{"status", "links", "internal", "volumes"},
	} {
		volumes, found, err := unstructured.NestedSlice(obj, fields...)
		if err != nil || !found {
			continue
		}
		if url := extractVMExportVolumeURL(volumes); url != "" {
			return url
		}
	}
	return ""
}

func extractVMExportVolumeURL(volumes []interface{}) string {
	var fallback string
	for _, volumeValue := range volumes {
		volume, ok := volumeValue.(map[string]interface{})
		if !ok {
			continue
		}
		formats, found, err := unstructured.NestedSlice(volume, "formats")
		if err != nil || !found {
			continue
		}
		for _, preferred := range []string{"gzip", "raw", "tar.gz", "dir"} {
			for _, formatValue := range formats {
				formatItem, ok := formatValue.(map[string]interface{})
				if !ok {
					continue
				}
				formatName, _, _ := unstructured.NestedString(formatItem, "format")
				url, _, _ := unstructured.NestedString(formatItem, "url")
				if url == "" {
					continue
				}
				if fallback == "" {
					fallback = url
				}
				if strings.EqualFold(formatName, preferred) {
					return url
				}
			}
		}
	}
	return fallback
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

func extractVMExportAuthSummary(obj map[string]interface{}) (string, bool, bool) {
	for _, candidate := range []struct {
		source string
		fields []string
	}{
		{source: "internal", fields: []string{"status", "links", "internal"}},
		{source: "external", fields: []string{"status", "links", "external"}},
	} {
		link, found, err := unstructured.NestedMap(obj, candidate.fields...)
		if err != nil || !found {
			continue
		}
		volumes, found, err := unstructured.NestedSlice(link, "volumes")
		if err != nil || !found || len(volumes) == 0 {
			continue
		}
		cert, _, _ := unstructured.NestedString(link, "cert")
		tokenSecretRef := extractVMExportTokenSecretRef(obj)
		return candidate.source, strings.TrimSpace(cert) != "", strings.TrimSpace(tokenSecretRef) != ""
	}
	return "", false, false
}

func extractVMExportTokenSecretRef(obj map[string]interface{}) string {
	if tokenSecretRef, _, _ := unstructured.NestedString(obj, "status", "tokenSecretRef"); strings.TrimSpace(tokenSecretRef) != "" {
		return tokenSecretRef
	}
	tokenSecretRef, _, _ := unstructured.NestedString(obj, "spec", "tokenSecretRef")
	return tokenSecretRef
}

func marshalVMExportStatus(status *VMExportStatus) string {
	if status == nil {
		return ""
	}
	data, _ := json.Marshal(status)
	return string(data)
}

func buildVMMachineManifest(arch, bootMode string, status *VMExportStatus, uploaded map[string]VMExportUploadedDisk) (*VMMachineManifest, string, error) {
	if status == nil {
		return nil, "", fmt.Errorf("vm export status is required")
	}
	if len(status.Disks) == 0 {
		return nil, "", fmt.Errorf("vm export disks are required")
	}
	if len(uploaded) == 0 {
		return nil, "", fmt.Errorf("uploaded disk metadata is required")
	}
	disks := make([]VMMachineManifestDisk, 0, len(status.Disks))
	rootObjectURI := ""
	rootDiskKey := ""
	for _, disk := range status.Disks {
		meta, ok := uploaded[disk.DiskKey]
		if !ok {
			return nil, "", fmt.Errorf("uploaded metadata missing for disk %s", disk.DiskKey)
		}
		disks = append(disks, VMMachineManifestDisk{
			DiskKey:   disk.DiskKey,
			DiskName:  firstNonEmptyString(disk.DiskName, disk.DiskKey),
			DiskRole:  firstNonEmptyString(disk.DiskRole, "data"),
			BootOrder: disk.BootOrder,
			ObjectKey: meta.ObjectKey,
			ObjectURI: meta.ObjectURI,
			Format:    meta.Format,
			SizeBytes: meta.SizeBytes,
			Checksum:  meta.Checksum,
		})
		if strings.EqualFold(disk.DiskRole, "root") {
			rootObjectURI = meta.ObjectURI
			rootDiskKey = disk.DiskKey
		}
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
	if rootDiskKey == "" {
		return nil, "", fmt.Errorf("root disk metadata is required")
	}
	return &VMMachineManifest{
		Version:     "v1",
		Arch:        strings.TrimSpace(arch),
		BootMode:    strings.TrimSpace(bootMode),
		RootDiskKey: rootDiskKey,
		Disks:       disks,
	}, rootObjectURI, nil
}

func buildVMAssetRestorePlan(manifest *VMMachineManifest, signer func(objectKey string) (string, error)) (*VMAssetRestorePlan, error) {
	if manifest == nil {
		return nil, fmt.Errorf("machine manifest is required")
	}
	if len(manifest.Disks) == 0 {
		return nil, fmt.Errorf("machine manifest disks are required")
	}
	if signer == nil {
		return nil, fmt.Errorf("restore url signer is required")
	}
	imports := make([]VMAssetRestoreDiskImport, 0, len(manifest.Disks))
	layout := make([]VMAssetRestoreDiskLayoutItem, 0, len(manifest.Disks))
	sorted := make([]VMMachineManifestDisk, len(manifest.Disks))
	copy(sorted, manifest.Disks)
	sort.Slice(sorted, func(i, j int) bool {
		iRank := vmExportDiskRoleRank(sorted[i].DiskRole)
		jRank := vmExportDiskRoleRank(sorted[j].DiskRole)
		if iRank == jRank {
			if sorted[i].BootOrder != 0 && sorted[j].BootOrder != 0 && sorted[i].BootOrder != sorted[j].BootOrder {
				return sorted[i].BootOrder < sorted[j].BootOrder
			}
			return sorted[i].DiskKey < sorted[j].DiskKey
		}
		return iRank < jRank
	})
	for index, disk := range sorted {
		signedURL, err := signer(disk.ObjectKey)
		if err != nil {
			return nil, err
		}
		volumeName := disk.DiskKey
		if strings.EqualFold(disk.DiskRole, "root") {
			volumeName = "disk"
		}
		imports = append(imports, VMAssetRestoreDiskImport{
			VolumeName: volumeName,
			DiskKey:    firstNonEmptyString(disk.DiskKey, volumeName),
			DiskName:   firstNonEmptyString(disk.DiskName, disk.DiskKey, volumeName),
			ImageURL:   signedURL,
			SourceURI:  firstNonEmptyString(disk.ObjectURI, disk.ObjectKey),
			Format:     disk.Format,
			Checksum:   disk.Checksum,
		})
		layout = append(layout, VMAssetRestoreDiskLayoutItem{
			DiskKey:    firstNonEmptyString(disk.DiskKey, volumeName),
			DiskName:   firstNonEmptyString(disk.DiskName, disk.DiskKey, volumeName),
			DiskRole:   firstNonEmptyString(disk.DiskRole, "data"),
			BootOrder:  disk.BootOrder,
			OrderIndex: index,
			Boot:       strings.EqualFold(disk.DiskRole, "root") || disk.BootOrder == 1 || disk.DiskKey == manifest.RootDiskKey,
		})
	}
	return &VMAssetRestorePlan{
		BootSourceFormat: "disk",
		DiskImports:      imports,
		DiskLayout:       layout,
	}, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
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
