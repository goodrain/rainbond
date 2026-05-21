package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

type VMExportRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type VMExportStatus struct {
	ExportName     string `json:"export_name"`
	Namespace      string `json:"namespace"`
	SourcePVC      string `json:"source_pvc"`
	TokenSecretRef string `json:"token_secret_ref"`
	Phase          string `json:"phase,omitempty"`
	DownloadURL    string `json:"download_url,omitempty"`
}

var vmExportDynamicClient = func() dynamic.Interface {
	if k8s.Default() == nil {
		return nil
	}
	return k8s.Default().DynamicClient
}

var vmExportGVR = schema.GroupVersionResource{
	Group:    "export.kubevirt.io",
	Version:  "v1beta1",
	Resource: "virtualmachineexports",
}

func (s *ServiceAction) CreateVMExport(serviceID string, req *VMExportRequest) (*VMExportStatus, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("export name is required")
	}
	if s == nil || s.kubevirtClient == nil {
		return nil, fmt.Errorf("kubevirt client is not initialized")
	}
	dynamicClient := vmExportDynamicClient()
	if dynamicClient == nil {
		return nil, fmt.Errorf("dynamic client is not initialized")
	}
	if s.kubeClient == nil {
		return nil, fmt.Errorf("kube client is not initialized")
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
	sourcePVC, err := resolveVMRootPVC(vm)
	if err != nil {
		return nil, err
	}
	tokenSecretRef := strings.TrimSpace(req.Name) + "-token"
	if err := ensureVMExportTokenSecret(s.kubeClient, vm.Namespace, tokenSecretRef); err != nil {
		return nil, err
	}
	resourceIf := dynamicClient.Resource(vmExportGVR).Namespace(vm.Namespace)
	_ = resourceIf.Delete(context.Background(), strings.TrimSpace(req.Name), metav1.DeleteOptions{})
	obj := buildVMExport(vm.Namespace, strings.TrimSpace(req.Name), strings.TrimSpace(req.Description), serviceID, sourcePVC, tokenSecretRef)
	created, err := resourceIf.Create(context.Background(), obj, metav1.CreateOptions{})
	if err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
		created, err = resourceIf.Get(context.Background(), strings.TrimSpace(req.Name), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}
	return &VMExportStatus{
		ExportName:     created.GetName(),
		Namespace:      created.GetNamespace(),
		SourcePVC:      sourcePVC,
		TokenSecretRef: tokenSecretRef,
		Phase:          nestedString(created.Object, "status", "phase"),
		DownloadURL:    extractVMExportDownloadURL(created.Object),
	}, nil
}

func (s *ServiceAction) GetVMExport(serviceID, exportName string) (*VMExportStatus, error) {
	if strings.TrimSpace(exportName) == "" {
		return nil, fmt.Errorf("export name is required")
	}
	if s == nil || s.kubevirtClient == nil {
		return nil, fmt.Errorf("kubevirt client is not initialized")
	}
	dynamicClient := vmExportDynamicClient()
	if dynamicClient == nil {
		return nil, fmt.Errorf("dynamic client is not initialized")
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
	export, err := dynamicClient.Resource(vmExportGVR).Namespace(vm.Namespace).Get(context.Background(), strings.TrimSpace(exportName), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return &VMExportStatus{
		ExportName:     export.GetName(),
		Namespace:      export.GetNamespace(),
		SourcePVC:      nestedString(export.Object, "spec", "source", "name"),
		TokenSecretRef: nestedString(export.Object, "spec", "tokenSecretRef"),
		Phase:          nestedString(export.Object, "status", "phase"),
		DownloadURL:    extractVMExportDownloadURL(export.Object),
	}, nil
}

func resolveVMRootPVC(vm *kubevirtv1.VirtualMachine) (string, error) {
	if vm == nil {
		return "", fmt.Errorf("vm is nil")
	}
	var rootDiskName string
	var bestOrder uint
	for _, disk := range vm.Spec.Template.Spec.Domain.Devices.Disks {
		if disk.BootOrder == nil {
			continue
		}
		if rootDiskName == "" || *disk.BootOrder < bestOrder {
			rootDiskName = disk.Name
			bestOrder = *disk.BootOrder
		}
	}
	if rootDiskName == "" && len(vm.Spec.Template.Spec.Domain.Devices.Disks) > 0 {
		rootDiskName = vm.Spec.Template.Spec.Domain.Devices.Disks[0].Name
	}
	if rootDiskName == "" {
		return "", fmt.Errorf("vm has no disks")
	}
	for _, volume := range vm.Spec.Template.Spec.Volumes {
		if volume.Name != rootDiskName {
			continue
		}
		if volume.DataVolume != nil && strings.TrimSpace(volume.DataVolume.Name) != "" {
			return volume.DataVolume.Name, nil
		}
		if volume.PersistentVolumeClaim != nil && strings.TrimSpace(volume.PersistentVolumeClaim.ClaimName) != "" {
			return volume.PersistentVolumeClaim.ClaimName, nil
		}
	}
	return "", fmt.Errorf("root disk %s has no pvc-backed volume source", rootDiskName)
}

func ensureVMExportTokenSecret(kubeClient kubernetes.Interface, namespace, secretName string) error {
	if kubeClient == nil {
		return fmt.Errorf("kube client is nil")
	}
	secrets := kubeClient.CoreV1().Secrets(namespace)
	existing, err := secrets.Get(context.Background(), secretName, metav1.GetOptions{})
	if err == nil && existing != nil {
		if existing.Data == nil {
			existing.Data = map[string][]byte{}
		}
		if len(existing.Data["token"]) == 0 {
			existing.Data["token"] = []byte(uuid.New().String())
			_, err = secrets.Update(context.Background(), existing, metav1.UpdateOptions{})
		}
		return err
	}
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	_, err = secrets.Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"token": []byte(uuid.New().String()),
		},
	}, metav1.CreateOptions{})
	return err
}

func buildVMExport(namespace, exportName, description, serviceID, sourcePVC, tokenSecretRef string) *unstructured.Unstructured {
	annotations := map[string]interface{}{}
	if description != "" {
		annotations["description"] = description
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "export.kubevirt.io/v1beta1",
			"kind":       "VirtualMachineExport",
			"metadata": map[string]interface{}{
				"name":      exportName,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"service_id": serviceID,
				},
				"annotations": annotations,
			},
			"spec": map[string]interface{}{
				"tokenSecretRef": tokenSecretRef,
				"ttlDuration":    "24h",
				"source": map[string]interface{}{
					"apiGroup": "",
					"kind":     "PersistentVolumeClaim",
					"name":     sourcePVC,
				},
			},
		},
	}
}

func nestedString(obj map[string]interface{}, fields ...string) string {
	current := obj
	for i, field := range fields {
		value, ok := current[field]
		if !ok {
			return ""
		}
		if i == len(fields)-1 {
			if s, ok := value.(string); ok {
				return s
			}
			return ""
		}
		next, ok := value.(map[string]interface{})
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}

func extractVMExportDownloadURL(obj map[string]interface{}) string {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return ""
	}
	links, ok := status["links"].(map[string]interface{})
	if !ok {
		return ""
	}
	internal, ok := links["internal"].(map[string]interface{})
	if !ok {
		return ""
	}
	volumes, ok := internal["volumes"].([]interface{})
	if !ok {
		return ""
	}
	var fallback string
	for _, volume := range volumes {
		volumeMap, ok := volume.(map[string]interface{})
		if !ok {
			continue
		}
		formats, ok := volumeMap["formats"].([]interface{})
		if !ok {
			continue
		}
		for _, formatItem := range formats {
			formatMap, ok := formatItem.(map[string]interface{})
			if !ok {
				continue
			}
			url, _ := formatMap["url"].(string)
			format, _ := formatMap["format"].(string)
			if url == "" {
				continue
			}
			if format == "gzip" {
				return url
			}
			if fallback == "" {
				fallback = url
			}
		}
	}
	return fallback
}
