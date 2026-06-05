package handler

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	kubecli "kubevirt.io/client-go/kubecli"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-export.root-disk-url
func TestResolveVMRootPVC(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							Disks: []kubevirtv1.Disk{
								{
									Name: "manual22",
									BootOrder: func() *uint {
										value := uint(1)
										return &value
									}(),
								},
							},
						},
					},
					Volumes: []kubevirtv1.Volume{
						{
							Name: "manual22",
							VolumeSource: kubevirtv1.VolumeSource{
								DataVolume: &kubevirtv1.DataVolumeSource{Name: "manual22"},
							},
						},
					},
				},
			},
		},
	}

	pvc, err := resolveVMRootPVC(vm)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pvc != "manual22" {
		t.Fatalf("expected manual22, got %s", pvc)
	}
}

func TestCreateVMExport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)
	vmExportDynamicClient = func() dynamic.Interface {
		return fake.NewSimpleDynamicClient(runtime.NewScheme())
	}
	defer func() {
		vmExportDynamicClient = func() dynamic.Interface {
			if k8s.Default() == nil {
				return nil
			}
			return k8s.Default().DynamicClient
		}
	}()

	mockClient.EXPECT().VirtualMachine("").Return(mockVMInterface)
	mockVMInterface.EXPECT().List(gomock.Any(), metav1.ListOptions{LabelSelector: "service_id=service-1"}).Return(&kubevirtv1.VirtualMachineList{
		Items: []kubevirtv1.VirtualMachine{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-vm",
					Namespace: "demo-ns",
				},
				Spec: kubevirtv1.VirtualMachineSpec{
					Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Domain: kubevirtv1.DomainSpec{
								Devices: kubevirtv1.Devices{
									Disks: []kubevirtv1.Disk{
										{
											Name: "manual22",
											BootOrder: func() *uint {
												value := uint(1)
												return &value
											}(),
										},
									},
								},
							},
							Volumes: []kubevirtv1.Volume{
								{
									Name: "manual22",
									VolumeSource: kubevirtv1.VolumeSource{
										DataVolume: &kubevirtv1.DataVolumeSource{Name: "manual22"},
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil)

	kubeClient := k8sfake.NewSimpleClientset()
	action := &ServiceAction{
		kubevirtClient: mockClient,
		kubeClient:     kubeClient,
	}

	status, err := action.CreateVMExport("service-1", &VMExportRequest{
		Name:        "osdisk-export",
		Description: "root disk export",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.ExportName != "osdisk-export" {
		t.Fatalf("unexpected export name: %#v", status)
	}
	if status.SourcePVC != "manual22" {
		t.Fatalf("unexpected source pvc: %#v", status)
	}
	secret, err := kubeClient.CoreV1().Secrets("demo-ns").Get(context.Background(), "osdisk-export-token", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected token secret, got %v", err)
	}
	if len(secret.Data["token"]) == 0 {
		t.Fatalf("expected token secret to contain token")
	}
	if status.DownloadToken != string(secret.Data["token"]) {
		t.Fatalf("expected export status to include download token")
	}
}

func TestExtractVMExportDownloadURLPrefersGzip(t *testing.T) {
	url := extractVMExportDownloadURL(map[string]interface{}{
		"status": map[string]interface{}{
			"links": map[string]interface{}{
				"internal": map[string]interface{}{
					"volumes": []interface{}{
						map[string]interface{}{
							"formats": []interface{}{
								map[string]interface{}{
									"format": "raw",
									"url":    "https://export.default.svc/volumes/root/disk.img",
								},
								map[string]interface{}{
									"format": "gzip",
									"url":    "https://export.default.svc/volumes/root/disk.img.gz",
								},
							},
						},
					},
				},
			},
		},
	})

	if url != "https://export.default.svc/volumes/root/disk.img.gz" {
		t.Fatalf("expected gzip url, got %s", url)
	}
}

func TestGetVMExport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	export := buildVMExport("demo-ns", "osdisk-export", "root disk export", "service-1", "manual22", "osdisk-export-token")
	export.Object["status"] = map[string]interface{}{
		"phase": "Ready",
		"links": map[string]interface{}{
			"internal": map[string]interface{}{
				"volumes": []interface{}{
					map[string]interface{}{
						"formats": []interface{}{
							map[string]interface{}{
								"format": "gzip",
								"url":    "https://virt-export-osdisk-export.default.svc/volumes/manual22/disk.img.gz",
							},
						},
					},
				},
			},
		},
	}
	vmExportDynamicClient = func() dynamic.Interface {
		return fake.NewSimpleDynamicClient(runtime.NewScheme(), export)
	}
	defer func() {
		vmExportDynamicClient = func() dynamic.Interface {
			if k8s.Default() == nil {
				return nil
			}
			return k8s.Default().DynamicClient
		}
	}()

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)
	mockClient.EXPECT().VirtualMachine("").Return(mockVMInterface)
	mockVMInterface.EXPECT().List(gomock.Any(), metav1.ListOptions{LabelSelector: "service_id=service-1"}).Return(&kubevirtv1.VirtualMachineList{
		Items: []kubevirtv1.VirtualMachine{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-vm",
					Namespace: "demo-ns",
				},
			},
		},
	}, nil)

	kubeClient := k8sfake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "osdisk-export-token",
			Namespace: "demo-ns",
		},
		Data: map[string][]byte{
			"token": []byte("download-token"),
		},
	})
	action := &ServiceAction{kubevirtClient: mockClient, kubeClient: kubeClient}
	status, err := action.GetVMExport("service-1", "osdisk-export")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.Phase != "Ready" {
		t.Fatalf("expected Ready phase, got %#v", status)
	}
	if status.SourcePVC != "manual22" {
		t.Fatalf("expected manual22 source pvc, got %#v", status)
	}
	if status.DownloadURL != "https://virt-export-osdisk-export.default.svc/volumes/manual22/disk.img.gz" {
		t.Fatalf("unexpected download url: %#v", status)
	}
	if status.DownloadToken != "download-token" {
		t.Fatalf("unexpected download token: %#v", status)
	}
}
