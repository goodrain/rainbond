package exector

import (
	"bytes"
	"testing"
	"time"

	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/sirupsen/logrus"
)

type stubTaskWorker struct {
	logger event.Logger
	runCh  chan struct{}
}

type stubEventManager struct{}

func (s *stubEventManager) GetLogger(eventID string) event.Logger {
	return event.NewLogger(eventID, make(chan []byte, 1))
}

func (s *stubEventManager) Start() error { return nil }

func (s *stubEventManager) Close() error { return nil }

func (s *stubEventManager) ReleaseLogger(logger event.Logger) {}

func (s *stubTaskWorker) Run(timeout time.Duration) error {
	close(s.runCh)
	return nil
}

func (s *stubTaskWorker) GetLogger() event.Logger {
	return s.logger
}

func (s *stubTaskWorker) Name() string {
	return "stub"
}

func (s *stubTaskWorker) Stop() error {
	return nil
}

func (s *stubTaskWorker) ErrorCallBack(err error) {}

// capability_id: rainbond.builder.registered-worker-dispatch
func TestRunTaskDoesNotWarnForRegisteredWorker(t *testing.T) {
	const taskType = "test-registered-worker"

	originalCreator, hadOriginalCreator := workerCreaterList[taskType]
	defer func() {
		if hadOriginalCreator {
			workerCreaterList[taskType] = originalCreator
			return
		}
		delete(workerCreaterList, taskType)
	}()

	runCh := make(chan struct{})
	RegisterWorker(taskType, func(in []byte, m *exectorManager) (TaskWorker, error) {
		return &stubTaskWorker{
			logger: event.NewLogger("test-registered-worker", make(chan []byte, 1)),
			runCh:  runCh,
		}, nil
	})
	event.NewTestManager(&stubEventManager{})
	defer event.NewTestManager(nil)

	manager := &exectorManager{
		tasks: make(chan *pb.TaskMessage, 1),
	}
	task := &pb.TaskMessage{TaskId: "task-1", TaskType: taskType}
	manager.tasks <- task

	var logs bytes.Buffer
	originalOut := logrus.StandardLogger().Out
	originalLevel := logrus.GetLevel()
	logrus.SetOutput(&logs)
	logrus.SetLevel(logrus.InfoLevel)
	defer func() {
		logrus.SetOutput(originalOut)
		logrus.SetLevel(originalLevel)
	}()

	manager.RunTask(task)

	select {
	case <-runCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for registered worker to run")
	}

	if got := logs.String(); bytes.Contains([]byte(got), []byte("Unknown task type")) {
		t.Fatalf("expected registered worker to avoid unknown task warning, got logs: %s", got)
	}
}

// capability_id: rainbond.vm-publish.resource-guard
func TestRunVMPublishTaskSerializesHeavyTasksAndHoldsQueueSlot(t *testing.T) {
	manager := &exectorManager{
		tasks:          make(chan *pb.TaskMessage, 2),
		vmPublishTasks: make(chan struct{}, 1),
	}
	first := &pb.TaskMessage{TaskId: "vm-publish-1", TaskType: "build_from_vm"}
	second := &pb.TaskMessage{TaskId: "vm-publish-2", TaskType: "build_from_vm"}
	manager.tasks <- first
	manager.tasks <- second

	started := make(chan string, 2)
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})
	run := func(task *pb.TaskMessage) {
		started <- task.TaskId
		if task.TaskId == first.TaskId {
			<-releaseFirst
			return
		}
		<-releaseSecond
	}

	go manager.runVMPublishTask(run, first)
	select {
	case got := <-started:
		if got != first.TaskId {
			t.Fatalf("expected first task to start, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first VM publish task to start")
	}

	go manager.runVMPublishTask(run, second)
	select {
	case got := <-started:
		t.Fatalf("second VM publish task started before first completed: %s", got)
	case <-time.After(100 * time.Millisecond):
	}
	if got := len(manager.tasks); got != 2 {
		t.Fatalf("expected running and waiting VM publish tasks to hold queue slots, got %d", got)
	}

	close(releaseFirst)
	select {
	case got := <-started:
		if got != second.TaskId {
			t.Fatalf("expected second task to start after first completed, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second VM publish task to start")
	}
	close(releaseSecond)

	deadline := time.After(2 * time.Second)
	for len(manager.tasks) != 0 {
		select {
		case <-deadline:
			t.Fatalf("expected all queue slots to be released, got %d", len(manager.tasks))
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// capability_id: rainbond.vm-publish.resource-guard
func TestIsVMPublishTaskDetectsVMShareImage(t *testing.T) {
	tests := []struct {
		name string
		task *pb.TaskMessage
		want bool
	}{
		{
			name: "direct vm build",
			task: &pb.TaskMessage{TaskType: "build_from_vm"},
			want: true,
		},
		{
			name: "vm share image",
			task: &pb.TaskMessage{
				TaskType: "share-image",
				TaskBody: []byte(`{
					"share_info": {
						"image_info": {
							"vm_image_source": "https://virt-export/default/disk.img.gz"
						}
					}
				}`),
			},
			want: true,
		},
		{
			name: "normal share image",
			task: &pb.TaskMessage{
				TaskType: "share-image",
				TaskBody: []byte(`{"share_info":{"image_info":{"image":"registry.example.com/demo:v1"}}}`),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isVMPublishTask(tt.task); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
