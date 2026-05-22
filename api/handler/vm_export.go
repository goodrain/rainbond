package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
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
	DownloadToken  string `json:"download_token,omitempty"`
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
	exportName := strings.TrimSpace(req.Name)
	logrus.Infof("[vm-export] create request: service_id=%s export_name=%s", serviceID, exportName)
	if s == nil || s.kubevirtClient == nil {
		logrus.Errorf("[vm-export] create failed before lookup: service_id=%s export_name=%s error=kubevirt client is not initialized", serviceID, exportName)
		return nil, fmt.Errorf("kubevirt client is not initialized")
	}
	dynamicClient := vmExportDynamicClient()
	if dynamicClient == nil {
		logrus.Errorf("[vm-export] create failed before lookup: service_id=%s export_name=%s error=dynamic client is not initialized", serviceID, exportName)
		return nil, fmt.Errorf("dynamic client is not initialized")
	}
	if s.kubeClient == nil {
		logrus.Errorf("[vm-export] create failed before lookup: service_id=%s export_name=%s error=kube client is not initialized", serviceID, exportName)
		return nil, fmt.Errorf("kube client is not initialized")
	}
	vms, err := s.kubevirtClient.VirtualMachine("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "service_id=" + serviceID,
	})
	if err != nil {
		logrus.Errorf("[vm-export] list vm failed: service_id=%s export_name=%s error=%v", serviceID, exportName, err)
		return nil, err
	}
	if len(vms.Items) == 0 {
		logrus.Warnf("[vm-export] vm not found: service_id=%s export_name=%s", serviceID, exportName)
		return nil, fmt.Errorf("service id is %v vm is not exist", serviceID)
	}
	vm := &vms.Items[0]
	logrus.Infof("[vm-export] vm resolved: service_id=%s export_name=%s vm_namespace=%s vm_name=%s", serviceID, exportName, vm.Namespace, vm.Name)
	sourcePVC, err := resolveVMRootPVC(vm)
	if err != nil {
		logrus.Errorf("[vm-export] resolve root pvc failed: service_id=%s export_name=%s vm_namespace=%s vm_name=%s error=%v", serviceID, exportName, vm.Namespace, vm.Name, err)
		return nil, err
	}
	tokenSecretRef := exportName + "-token"
	logrus.Infof("[vm-export] root pvc resolved: service_id=%s export_name=%s source_pvc=%s token_secret=%s", serviceID, exportName, sourcePVC, tokenSecretRef)
	if err := ensureVMExportTokenSecret(s.kubeClient, vm.Namespace, tokenSecretRef); err != nil {
		logrus.Errorf("[vm-export] ensure token secret failed: service_id=%s export_name=%s namespace=%s secret=%s error=%v", serviceID, exportName, vm.Namespace, tokenSecretRef, err)
		return nil, err
	}
	downloadToken, err := readVMExportDownloadToken(s.kubeClient, vm.Namespace, tokenSecretRef)
	if err != nil {
		logrus.Errorf("[vm-export] read token secret failed: service_id=%s export_name=%s namespace=%s secret=%s error=%v", serviceID, exportName, vm.Namespace, tokenSecretRef, err)
		return nil, err
	}
	resourceIf := dynamicClient.Resource(vmExportGVR).Namespace(vm.Namespace)
	_ = resourceIf.Delete(context.Background(), exportName, metav1.DeleteOptions{})
	obj := buildVMExport(vm.Namespace, exportName, strings.TrimSpace(req.Description), serviceID, sourcePVC, tokenSecretRef)
	logrus.Infof("[vm-export] creating VirtualMachineExport: service_id=%s export_name=%s namespace=%s source_pvc=%s", serviceID, exportName, vm.Namespace, sourcePVC)
	created, err := resourceIf.Create(context.Background(), obj, metav1.CreateOptions{})
	if err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			logrus.Errorf("[vm-export] create VirtualMachineExport failed: service_id=%s export_name=%s namespace=%s error=%v", serviceID, exportName, vm.Namespace, err)
			return nil, err
		}
		logrus.Warnf("[vm-export] VirtualMachineExport already exists, reading existing object: service_id=%s export_name=%s namespace=%s", serviceID, exportName, vm.Namespace)
		created, err = resourceIf.Get(context.Background(), exportName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("[vm-export] get existing VirtualMachineExport failed: service_id=%s export_name=%s namespace=%s error=%v", serviceID, exportName, vm.Namespace, err)
			return nil, err
		}
	}
	status := &VMExportStatus{
		ExportName:     created.GetName(),
		Namespace:      created.GetNamespace(),
		SourcePVC:      sourcePVC,
		TokenSecretRef: tokenSecretRef,
		DownloadToken:  downloadToken,
		Phase:          nestedString(created.Object, "status", "phase"),
		DownloadURL:    extractVMExportDownloadURL(created.Object),
	}
	logrus.Infof("[vm-export] create response: service_id=%s export_name=%s namespace=%s phase=%s has_download_url=%t", serviceID, status.ExportName, status.Namespace, status.Phase, status.DownloadURL != "")
	return status, nil
}

func (s *ServiceAction) GetVMExport(serviceID, exportName string) (*VMExportStatus, error) {
	if strings.TrimSpace(exportName) == "" {
		return nil, fmt.Errorf("export name is required")
	}
	exportName = strings.TrimSpace(exportName)
	logrus.Infof("[vm-export] get request: service_id=%s export_name=%s", serviceID, exportName)
	if s == nil || s.kubevirtClient == nil {
		logrus.Errorf("[vm-export] get failed before lookup: service_id=%s export_name=%s error=kubevirt client is not initialized", serviceID, exportName)
		return nil, fmt.Errorf("kubevirt client is not initialized")
	}
	dynamicClient := vmExportDynamicClient()
	if dynamicClient == nil {
		logrus.Errorf("[vm-export] get failed before lookup: service_id=%s export_name=%s error=dynamic client is not initialized", serviceID, exportName)
		return nil, fmt.Errorf("dynamic client is not initialized")
	}
	vms, err := s.kubevirtClient.VirtualMachine("").List(context.Background(), metav1.ListOptions{
		LabelSelector: "service_id=" + serviceID,
	})
	if err != nil {
		logrus.Errorf("[vm-export] list vm failed: service_id=%s export_name=%s error=%v", serviceID, exportName, err)
		return nil, err
	}
	if len(vms.Items) == 0 {
		logrus.Warnf("[vm-export] vm not found: service_id=%s export_name=%s", serviceID, exportName)
		return nil, fmt.Errorf("service id is %v vm is not exist", serviceID)
	}
	if s.kubeClient == nil {
		logrus.Errorf("[vm-export] get failed before reading token: service_id=%s export_name=%s error=kube client is not initialized", serviceID, exportName)
		return nil, fmt.Errorf("kube client is not initialized")
	}
	vm := &vms.Items[0]
	export, err := dynamicClient.Resource(vmExportGVR).Namespace(vm.Namespace).Get(context.Background(), exportName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("[vm-export] get VirtualMachineExport failed: service_id=%s export_name=%s namespace=%s error=%v", serviceID, exportName, vm.Namespace, err)
		return nil, err
	}
	tokenSecretRef := nestedString(export.Object, "spec", "tokenSecretRef")
	downloadToken, err := readVMExportDownloadToken(s.kubeClient, vm.Namespace, tokenSecretRef)
	if err != nil {
		logrus.Errorf("[vm-export] read token secret failed: service_id=%s export_name=%s namespace=%s secret=%s error=%v", serviceID, exportName, vm.Namespace, tokenSecretRef, err)
		return nil, err
	}
	status := &VMExportStatus{
		ExportName:     export.GetName(),
		Namespace:      export.GetNamespace(),
		SourcePVC:      nestedString(export.Object, "spec", "source", "name"),
		TokenSecretRef: tokenSecretRef,
		DownloadToken:  downloadToken,
		Phase:          nestedString(export.Object, "status", "phase"),
		DownloadURL:    extractVMExportDownloadURL(export.Object),
	}
	logrus.Infof("[vm-export] get response: service_id=%s export_name=%s namespace=%s phase=%s has_download_url=%t", serviceID, status.ExportName, status.Namespace, status.Phase, status.DownloadURL != "")
	return status, nil
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

func readVMExportDownloadToken(kubeClient kubernetes.Interface, namespace, secretName string) (string, error) {
	if kubeClient == nil {
		return "", fmt.Errorf("kube client is nil")
	}
	if strings.TrimSpace(secretName) == "" {
		return "", fmt.Errorf("token secret is empty")
	}
	secret, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	token := string(secret.Data["token"])
	if strings.TrimSpace(token) == "" {
		return "", fmt.Errorf("token secret %s has no token", secretName)
	}
	return token, nil
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
