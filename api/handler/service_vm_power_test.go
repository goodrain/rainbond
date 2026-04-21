// capability_id: rainbond.vm-power.start-existing-or-create
package handler

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	apimodel "github.com/goodrain/rainbond/api/model"
	mqpb "github.com/goodrain/rainbond/mq/api/grpc/pb"
	mqclient "github.com/goodrain/rainbond/mq/client"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
)

type recordingMQClient struct {
	tasks []mqclient.TaskStruct
}

func (m *recordingMQClient) Enqueue(context.Context, *mqpb.EnqueueRequest, ...grpc.CallOption) (*mqpb.TaskReply, error) {
	return nil, nil
}

func (m *recordingMQClient) Topics(context.Context, *mqpb.TopicRequest, ...grpc.CallOption) (*mqpb.TaskReply, error) {
	return nil, nil
}

func (m *recordingMQClient) Dequeue(context.Context, *mqpb.DequeueRequest, ...grpc.CallOption) (*mqpb.TaskMessage, error) {
	return nil, nil
}

func (m *recordingMQClient) Close() {}

func (m *recordingMQClient) SendBuilderTopic(t mqclient.TaskStruct) error {
	m.tasks = append(m.tasks, t)
	return nil
}

func TestStartOrCreateVMStartsExistingStoppedVM(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)
	mq := &recordingMQClient{}

	mockClient.EXPECT().VirtualMachine("").Return(mockVMInterface)
	mockVMInterface.EXPECT().List(gomock.Any(), metav1.ListOptions{LabelSelector: "service_id=service-1"}).Return(&kubevirtv1.VirtualMachineList{
		Items: []kubevirtv1.VirtualMachine{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-vm",
					Namespace: "demo-ns",
				},
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusStopped,
				},
			},
		},
	}, nil)
	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMInterface)
	mockVMInterface.EXPECT().Start(gomock.Any(), "demo-vm", gomock.Any()).Return(nil)

	action := &ServiceAction{MQClient: mq, kubevirtClient: mockClient}
	err := action.StartOrCreateVM(&apimodel.StartStopStruct{
		TenantID:  "tenant-1",
		ServiceID: "service-1",
		EventID:   "event-1",
		TaskType:  "start",
	}, "deploy-v1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(mq.tasks) != 0 {
		t.Fatalf("expected no worker start task, got %#v", mq.tasks)
	}
}

func TestStartOrCreateVMFallsBackToWorkerStartWhenVMIsMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)
	mq := &recordingMQClient{}

	mockClient.EXPECT().VirtualMachine("").Return(mockVMInterface)
	mockVMInterface.EXPECT().List(gomock.Any(), metav1.ListOptions{LabelSelector: "service_id=service-1"}).Return(&kubevirtv1.VirtualMachineList{}, nil)

	action := &ServiceAction{MQClient: mq, kubevirtClient: mockClient}
	err := action.StartOrCreateVM(&apimodel.StartStopStruct{
		TenantID:  "tenant-1",
		ServiceID: "service-1",
		EventID:   "event-1",
		TaskType:  "start",
	}, "deploy-v1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(mq.tasks) != 1 {
		t.Fatalf("expected one worker start task, got %#v", mq.tasks)
	}
	if mq.tasks[0].TaskType != "start" {
		t.Fatalf("expected worker task type start, got %q", mq.tasks[0].TaskType)
	}
}

func TestStopVMStopsExistingVMWithoutWorkerTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)
	mq := &recordingMQClient{}

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
	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMInterface)
	mockVMInterface.EXPECT().Stop(gomock.Any(), "demo-vm", gomock.Any()).Return(nil)

	action := &ServiceAction{MQClient: mq, kubevirtClient: mockClient}
	err := action.StopVM("service-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(mq.tasks) != 0 {
		t.Fatalf("expected no worker stop task, got %#v", mq.tasks)
	}
}

func TestRestartVMRestartsExistingRunningVMWithoutWorkerTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)
	mq := &recordingMQClient{}

	mockClient.EXPECT().VirtualMachine("").Return(mockVMInterface)
	mockVMInterface.EXPECT().List(gomock.Any(), metav1.ListOptions{LabelSelector: "service_id=service-1"}).Return(&kubevirtv1.VirtualMachineList{
		Items: []kubevirtv1.VirtualMachine{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-vm",
					Namespace: "demo-ns",
				},
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusRunning,
				},
			},
		},
	}, nil)
	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMInterface)
	mockVMInterface.EXPECT().Restart(gomock.Any(), "demo-vm", gomock.Any()).Return(nil)

	action := &ServiceAction{MQClient: mq, kubevirtClient: mockClient}
	err := action.RestartVM(&apimodel.StartStopStruct{
		TenantID:  "tenant-1",
		ServiceID: "service-1",
		EventID:   "event-1",
		TaskType:  "restart",
	}, "deploy-v1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(mq.tasks) != 0 {
		t.Fatalf("expected no worker restart task, got %#v", mq.tasks)
	}
}

func TestRestartVMStartsStoppedVMWithoutWorkerTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)
	mq := &recordingMQClient{}

	mockClient.EXPECT().VirtualMachine("").Return(mockVMInterface)
	mockVMInterface.EXPECT().List(gomock.Any(), metav1.ListOptions{LabelSelector: "service_id=service-1"}).Return(&kubevirtv1.VirtualMachineList{
		Items: []kubevirtv1.VirtualMachine{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-vm",
					Namespace: "demo-ns",
				},
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusStopped,
				},
			},
		},
	}, nil)
	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMInterface)
	mockVMInterface.EXPECT().Start(gomock.Any(), "demo-vm", gomock.Any()).Return(nil)

	action := &ServiceAction{MQClient: mq, kubevirtClient: mockClient}
	err := action.RestartVM(&apimodel.StartStopStruct{
		TenantID:  "tenant-1",
		ServiceID: "service-1",
		EventID:   "event-1",
		TaskType:  "restart",
	}, "deploy-v1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(mq.tasks) != 0 {
		t.Fatalf("expected no worker restart task, got %#v", mq.tasks)
	}
}
