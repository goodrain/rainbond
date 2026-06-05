package handler

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	kubecli "kubevirt.io/client-go/kubecli"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	kubevirtv1 "kubevirt.io/api/core/v1"
	snapshotv1 "kubevirt.io/api/snapshot/v1beta1"
)

type snapshotClientStub struct {
	created *snapshotv1.VirtualMachineSnapshot
}

func (s *snapshotClientStub) Create(_ context.Context, snapshot *snapshotv1.VirtualMachineSnapshot, _ metav1.CreateOptions) (*snapshotv1.VirtualMachineSnapshot, error) {
	s.created = snapshot
	return snapshot, nil
}

func (s *snapshotClientStub) Update(_ context.Context, snapshot *snapshotv1.VirtualMachineSnapshot, _ metav1.UpdateOptions) (*snapshotv1.VirtualMachineSnapshot, error) {
	return snapshot, nil
}

func (s *snapshotClientStub) UpdateStatus(_ context.Context, snapshot *snapshotv1.VirtualMachineSnapshot, _ metav1.UpdateOptions) (*snapshotv1.VirtualMachineSnapshot, error) {
	return snapshot, nil
}

func (s *snapshotClientStub) Delete(_ context.Context, _ string, _ metav1.DeleteOptions) error { return nil }
func (s *snapshotClientStub) DeleteCollection(_ context.Context, _ metav1.DeleteOptions, _ metav1.ListOptions) error {
	return nil
}
func (s *snapshotClientStub) Get(_ context.Context, _ string, _ metav1.GetOptions) (*snapshotv1.VirtualMachineSnapshot, error) {
	return nil, nil
}
func (s *snapshotClientStub) List(_ context.Context, _ metav1.ListOptions) (*snapshotv1.VirtualMachineSnapshotList, error) {
	return &snapshotv1.VirtualMachineSnapshotList{}, nil
}
func (s *snapshotClientStub) Watch(_ context.Context, _ metav1.ListOptions) (watch.Interface, error) { return nil, nil }
func (s *snapshotClientStub) Patch(_ context.Context, _ string, _ types.PatchType, _ []byte, _ metav1.PatchOptions, _ ...string) (*snapshotv1.VirtualMachineSnapshot, error) {
	return nil, nil
}

func TestBuildVMSnapshot(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
	}

	snapshot := buildVMSnapshot(vm, "snap-1", "demo snapshot", "service-1")

	if snapshot.Name != "snap-1" {
		t.Fatalf("expected snapshot name snap-1, got %s", snapshot.Name)
	}
	if snapshot.Namespace != "demo-ns" {
		t.Fatalf("expected namespace demo-ns, got %s", snapshot.Namespace)
	}
	if snapshot.Spec.Source.Kind != "VirtualMachine" || snapshot.Spec.Source.Name != "demo-vm" {
		t.Fatalf("unexpected snapshot source %#v", snapshot.Spec.Source)
	}
	if snapshot.Labels["service_id"] != "service-1" {
		t.Fatalf("expected service label, got %#v", snapshot.Labels)
	}
}

func TestCreateVMSnapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)
	snapshotClient := &snapshotClientStub{}

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
	mockClient.EXPECT().VirtualMachineSnapshot("demo-ns").Return(snapshotClient)

	action := &ServiceAction{kubevirtClient: mockClient}
	status, err := action.CreateVMSnapshot("service-1", &VMSnapshotRequest{
		Name:        "snap-1",
		Description: "demo snapshot",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.SnapshotName != "snap-1" {
		t.Fatalf("expected snapshot name snap-1, got %#v", status)
	}
	if snapshotClient.created == nil {
		t.Fatal("expected snapshot to be created")
	}
	if snapshotClient.created.Spec.Source.Kind != "VirtualMachine" || snapshotClient.created.Spec.Source.Name != "demo-vm" {
		t.Fatalf("unexpected created snapshot source %#v", snapshotClient.created.Spec.Source)
	}
}
