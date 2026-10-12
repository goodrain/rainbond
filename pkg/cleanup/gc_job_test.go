package cleanup

import (
	"errors"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

func TestGCJobIntentAndIdentitySurviveRestartWithoutReplay(t *testing.T) {
	database, path := coordinationDB(t)
	request := operation("gc-job", "gc", "*")
	hash := strings.Repeat("a", 64)
	if _, _, err := PrepareGCJob(database, request, "system", hash); err == nil {
		t.Fatal("job prepared without a maintenance request")
	}
	if _, err := RequestMaintenance(database, request); err != nil {
		t.Fatal(err)
	}
	intent, created, err := PrepareGCJob(database, request, "system", hash)
	if err != nil || !created || intent.Namespace != "system" || intent.Name == "" {
		t.Fatal(intent, created, err)
	}
	restarted, err := gorm.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restarted.LogMode(false)
	again, created, err := PrepareGCJob(restarted, request, "system", hash)
	if err != nil || created || again != intent {
		t.Fatal("creation intent replayed", again, created, err)
	}
	if _, _, err := PrepareGCJob(restarted, request, "different", hash); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("namespace replaced", err)
	}
	if _, _, err := PrepareGCJob(restarted, request, "system", strings.Repeat("b", 64)); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("spec replaced", err)
	}
	if err := BindGCJob(restarted, request, intent, "job-uid"); err != nil {
		t.Fatal(err)
	}
	if err := BindGCJob(restarted, request, intent, "job-uid"); err != nil {
		t.Fatal("same binding not idempotent", err)
	}
	if err := BindGCJob(restarted, request, intent, "replacement-uid"); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("replacement Job adopted", err)
	}
	saved, err := ReadGCJobBinding(restarted, request)
	if err != nil || saved.JobUID != "job-uid" || saved.Name != intent.Name {
		t.Fatal(saved, err)
	}
}

func TestGCJobExecutionRequiresBoundPodAndDrainedWriters(t *testing.T) {
	database, _ := coordinationDB(t)
	writer := operation("writer", "producer", "app")
	if _, err := AcquireOperation(database, writer); err != nil {
		t.Fatal(err)
	}
	request := operation("gc-job", "gc", "*")
	if _, err := RequestMaintenance(database, request); err != nil {
		t.Fatal(err)
	}
	intent, _, err := PrepareGCJob(database, request, "system", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if err := EnterMaintenance(database, request); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("generic entry bypassed job binding", err)
	}
	if err := BindGCExecutor(database, request, "job-uid", "pod-name", "pod-uid"); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("Pod bound before Job", err)
	}
	if err := BindGCJob(database, request, intent, "job-uid"); err != nil {
		t.Fatal(err)
	}
	if err := BindGCExecutor(database, request, "job-uid", "pod-name", "pod-uid"); err != nil {
		t.Fatal(err)
	}
	if err := BindGCExecutor(database, request, "job-uid", "replacement-name", "replacement-pod"); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("replacement Pod adopted", err)
	}
	if err := EnterGCJobExecution(database, request, "job-uid", "wrong-pod"); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("wrong Pod granted execution", err)
	}
	if err := EnterGCJobExecution(database, request, "job-uid", "pod-uid"); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("active writer ignored", err)
	}
	if err := FinishOperation(database, writer, true); err != nil {
		t.Fatal(err)
	}
	if err := EnterGCJobExecution(database, request, "job-uid", "pod-uid"); err != nil {
		t.Fatal(err)
	}
	if err := EnterGCJobExecution(database, request, "job-uid", "pod-uid"); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("GC execution granted twice", err)
	}
	var stored model.CleanupOperation
	if err := database.Where("operation_id = ?", request.OperationID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.State != "exclusive" {
		t.Fatal(stored.State)
	}
	if err := FinishMaintenanceRestore(database, request, true); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("job binding restored writes without GC result", err)
	}
}

func TestGCJobInvalidIntentAndCancellationCannotGrantExecution(t *testing.T) {
	database, _ := coordinationDB(t)
	request := operation("gc-job", "gc", "*")
	if _, err := RequestMaintenance(database, request); err != nil {
		t.Fatal(err)
	}
	for _, value := range []struct{ namespace, hash string }{{"../system", strings.Repeat("a", 64)}, {"system", ""}, {"system", strings.Repeat("A", 64)}} {
		if _, _, err := PrepareGCJob(database, request, value.namespace, value.hash); !errors.Is(err, ErrCoordinationChanged) {
			t.Fatal("invalid intent accepted", err)
		}
	}
	intent, _, err := PrepareGCJob(database, request, "system", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if err := BindGCJob(database, request, intent, "job-uid"); err != nil {
		t.Fatal(err)
	}
	if err := BindGCExecutor(database, request, "job-uid", "pod-name", "pod-uid"); err != nil {
		t.Fatal(err)
	}
	if err := CancelMaintenanceDrain(database, request); err != nil {
		t.Fatal(err)
	}
	if err := EnterGCJobExecution(database, request, "job-uid", "pod-uid"); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("canceled operation started GC", err)
	}
}

func TestConcurrentGCPreparationAndEntryGrantOnce(t *testing.T) {
	database, _ := coordinationDB(t)
	request := operation("concurrent-gc", "gc", "*")
	if _, err := RequestMaintenance(database, request); err != nil {
		t.Fatal(err)
	}
	type preparation struct {
		intent  GCJobIntent
		created bool
		err     error
	}
	prepared := make(chan preparation, 2)
	for i := 0; i < 2; i++ {
		go func() {
			intent, created, err := PrepareGCJob(database, request, "system", strings.Repeat("a", 64))
			prepared <- preparation{intent, created, err}
		}()
	}
	grants := 0
	var intent GCJobIntent
	for i := 0; i < 2; i++ {
		result := <-prepared
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.created {
			grants++
		}
		intent = result.intent
	}
	if grants != 1 {
		t.Fatalf("creation grants=%d", grants)
	}
	if err := BindGCJob(database, request, intent, "job-uid"); err != nil {
		t.Fatal(err)
	}
	if err := BindGCExecutor(database, request, "job-uid", "pod-name", "pod-uid"); err != nil {
		t.Fatal(err)
	}
	entered := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { entered <- EnterGCJobExecution(database, request, "job-uid", "pod-uid") }()
	}
	grants = 0
	for i := 0; i < 2; i++ {
		err := <-entered
		if err == nil {
			grants++
		} else if !errors.Is(err, ErrCoordinationChanged) {
			t.Fatal(err)
		}
	}
	if grants != 1 {
		t.Fatalf("execution grants=%d", grants)
	}
}
