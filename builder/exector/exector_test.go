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
