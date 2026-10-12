package exector

import (
	"errors"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
)

type blockedShareResult struct {
	dao.KeyValueDao
	entered, release chan struct{}
}

func (s *blockedShareResult) Put(string, string) error {
	close(s.entered)
	<-s.release
	return nil
}

type shareBoundaryDB struct {
	db.Manager
	results *blockedShareResult
}

func (s shareBoundaryDB) KeyValueDao() dao.KeyValueDao { return s.results }

type shareBoundaryLogs struct {
	vmActivationLogs
	finished chan struct{}
}

func (s *shareBoundaryLogs) ReleaseLogger(event.Logger) { close(s.finished) }

// capability_id: rainbond.cleanup.share-task-completion
func TestSlugShareTaskWaitsForResultPersistence(t *testing.T) {
	results := &blockedShareResult{entered: make(chan struct{}), release: make(chan struct{})}
	db.SetTestManager(shareBoundaryDB{results: results})
	defer db.SetTestManager(nil)
	logs := &shareBoundaryLogs{finished: make(chan struct{})}
	event.NewTestManager(logs)
	defer event.NewTestManager(nil)
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		(&exectorManager{}).slugShare(&pb.TaskMessage{TaskBody: []byte(`{"share_id":"boundary-test","share_info":{"event_id":"boundary-event"}}`)})
	}()
	defer func() {
		close(results.release)
		select {
		case <-logs.finished:
		case <-time.After(5 * time.Second):
			t.Error("share task did not release its logger")
		}
		select {
		case <-returned:
		case <-time.After(5 * time.Second):
			t.Error("share task did not terminate after persistence completed")
		}
	}()
	select {
	case <-results.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("share result was never persisted")
	}
	select {
	case <-returned:
		t.Fatal("task returned while share result persistence was still in flight")
	case <-time.After(50 * time.Millisecond):
	}
}

type blockedMarketCompletionLogs struct {
	vmActivationLogs
	entered, release chan struct{}
}

func (s *blockedMarketCompletionLogs) ReleaseLogger(event.Logger) {
	close(s.entered)
	<-s.release
}

// capability_id: rainbond.cleanup.market-slug-task-completion
func TestMarketSlugTaskWaitsForCompletion(t *testing.T) {
	// Invalid TCP port rejects before dialing; do not access a user's SSH agent.
	t.Setenv("SSH_AUTH_SOCK", "")
	logs := &blockedMarketCompletionLogs{entered: make(chan struct{}), release: make(chan struct{})}
	event.NewTestManager(logs)
	defer event.NewTestManager(nil)
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		(&exectorManager{}).buildFromMarketSlug(&pb.TaskMessage{TaskBody: []byte(`{"event_id":"owned-market-boundary","slug_info":{"ftp_host":"127.0.0.1","ftp_port":"65536"}}`)})
	}()
	defer func() {
		close(logs.release)
		select {
		case <-returned:
		case <-time.After(5 * time.Second):
			t.Error("market task did not complete")
		}
	}()
	select {
	case <-logs.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("market task did not reach completion")
	}
	select {
	case <-returned:
		t.Fatal("market task returned while completion was still in flight")
	case <-time.After(50 * time.Millisecond):
	}
}

type failedImageShareStore struct {
	dao.KeyValueDao
	failure error
}

func (s failedImageShareStore) Put(string, string) error { return s.failure }

type imageShareStoreDB struct {
	db.Manager
	store dao.KeyValueDao
}

func (d imageShareStoreDB) KeyValueDao() dao.KeyValueDao { return d.store }
func TestImageShareCannotReportSuccessWhenReceiptPersistenceFails(t *testing.T) {
	failure := errors.New("owned persistence failure")
	db.SetTestManager(imageShareStoreDB{store: failedImageShareStore{failure: failure}})
	defer db.SetTestManager(nil)
	item := &ImageShareItem{ShareID: "owned-publication", Logger: event.NewLogger("owned-event", make(chan []byte, 20))}
	if err := item.UpdateShareStatus("success"); !errors.Is(err, failure) {
		t.Fatal("lost share receipt reported success", err)
	}
}

func TestPluginShareCannotReportSuccessWhenReceiptPersistenceFails(t *testing.T) {
	failure := errors.New("owned persistence failure")
	db.SetTestManager(imageShareStoreDB{store: failedImageShareStore{failure: failure}})
	defer db.SetTestManager(nil)
	item := &PluginShareItem{ShareID: "owned-plugin-publication", Logger: event.NewLogger("owned-event", make(chan []byte, 20))}
	if err := item.updateShareStatus("success"); !errors.Is(err, failure) {
		t.Fatal("lost plugin share receipt reported success", err)
	}
}

type failingCleanupWorker struct {
	stubTaskWorker
	failure error
}

func (w *failingCleanupWorker) Run(time.Duration) error { return w.failure }
func TestRegisteredWorkerReturnsExecutionFailure(t *testing.T) {
	failure := errors.New("owned worker failure")
	taskType := "owned-failing-cleanup-worker"
	RegisterWorker(taskType, func([]byte, *exectorManager) (TaskWorker, error) {
		return &failingCleanupWorker{stubTaskWorker: stubTaskWorker{logger: event.NewLogger("owned-event", make(chan []byte, 20))}, failure: failure}, nil
	})
	defer delete(workerCreaterList, taskType)
	event.NewTestManager(&vmActivationLogs{})
	defer event.NewTestManager(nil)
	if err := (&exectorManager{}).exec(&pb.TaskMessage{TaskId: "owned-task", TaskType: taskType}); !errors.Is(err, failure) {
		t.Fatal("failed worker reported success", err)
	}
}
