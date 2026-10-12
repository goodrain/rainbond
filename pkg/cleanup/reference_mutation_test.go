package cleanup

import (
	"errors"
	"testing"

	"github.com/jinzhu/gorm"
)

func TestReferenceMutationSerializesWithSelectedDeletion(t *testing.T) {
	database, _ := coordinationDB(t)
	selected := operation("selected", "delete", "app/a")
	if _, err := AcquireOperation(database, selected); err != nil {
		t.Fatal(err)
	}
	called := false
	write := func(*gorm.DB) error { called = true; return nil }
	if err := WithReferenceMutation(database, []string{"app/a"}, write); !errors.Is(err, ErrCoordinationBusy) || called {
		t.Fatal("reference created while its manifest is being deleted", err)
	}
	if err := WithReferenceMutation(database, []string{"app/b"}, write); err != nil || !called {
		t.Fatal("unrelated reference blocked", err)
	}
	called = false
	if err := WithReferenceMutation(database, nil, write); !errors.Is(err, ErrCoordinationBusy) || called {
		t.Fatal("unknown reference bypassed deletion protection", err)
	}
	if err := FinishOperation(database, selected, false); err != nil {
		t.Fatal(err)
	}
	if err := WithReferenceMutation(database, []string{"app/a"}, write); !errors.Is(err, ErrCoordinationBusy) || called {
		t.Fatal("uncertain deletion lost reference protection", err)
	}
}

func TestReferenceMutationRollsBackMetadataAndRevisionTogether(t *testing.T) {
	database, _ := coordinationDB(t)
	if err := database.Exec("CREATE TABLE test_references (name TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	before, err := InspectStorage(database, "store", "generation1")
	if err != nil {
		t.Fatal(err)
	}
	failed := errors.New("metadata persistence failed")
	err = WithReferenceMutation(database, []string{"app/a"}, func(tx *gorm.DB) error {
		if err := tx.Exec("INSERT INTO test_references (name) VALUES (?)", "selected").Error; err != nil {
			return err
		}
		return failed
	})
	if !errors.Is(err, failed) {
		t.Fatal(err)
	}
	var count int
	if err := database.Table("test_references").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("failed metadata write committed", count, err)
	}
	after, err := InspectStorage(database, "store", "generation1")
	if err != nil || after.Revision != before.Revision {
		t.Fatal("failed write advanced reference revision", after, err)
	}
}

func TestReferenceMutationUsesCallerTransactionWithoutCommittingIt(t *testing.T) {
	database, _ := coordinationDB(t)
	if err := database.Exec("CREATE TABLE test_references (name TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	tx := database.Begin()
	defer tx.Rollback()
	if err := WithReferenceMutation(tx, []string{"app/a"}, func(write *gorm.DB) error {
		return write.Exec("INSERT INTO test_references (name) VALUES (?)", "restore").Error
	}); err != nil {
		t.Fatal("restore transaction could not register reference", err)
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatal("reference helper committed its caller transaction", err)
	}
	var count int
	if err := database.Table("test_references").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("restoration rollback lost", count, err)
	}
}

func TestAdmittedProducerCanCommitDuringDrainWithoutOpeningAdmission(t *testing.T) {
	database, _ := coordinationDB(t)
	producer := operation("building", "producer", "app/a")
	if _, err := AcquireOperation(database, producer); err != nil {
		t.Fatal(err)
	}
	gc := operation("gc", "gc", "*")
	if _, err := RequestMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
	called := false
	write := func(tx *gorm.DB) error {
		return WithReferenceMutation(tx, []string{"app/a"}, func(*gorm.DB) error { called = true; return nil })
	}
	if err := WithProducerReferenceMutation(database, []CoordinationRequest{producer}, []string{"app/a"}, write); err != nil || !called {
		t.Fatal("admitted producer cannot finish metadata", err)
	}
	called = false
	if err := WithReferenceMutation(database, []string{"app/a"}, write); !errors.Is(err, ErrCoordinationBusy) || called {
		t.Fatal("new producer bypassed drain", err)
	}
	if err := EnterMaintenance(database, gc); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("metadata commit released producer prematurely", err)
	}
	if err := WithProducerReferenceMutation(database, []CoordinationRequest{producer}, []string{"app/b"}, write); err == nil || called {
		t.Fatal("producer expanded its scope")
	}
	forged := producer
	forged.Owner = "other"
	if err := WithProducerReferenceMutation(database, []CoordinationRequest{forged}, []string{"app/a"}, write); err == nil || called {
		t.Fatal("foreign owner used admission")
	}
	if err := FinishOperation(database, producer, true); err != nil {
		t.Fatal(err)
	}
	if err := WithProducerReferenceMutation(database, []CoordinationRequest{producer}, []string{"app/a"}, write); err == nil || called {
		t.Fatal("finished producer reused admission")
	}
	if err := EnterMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
}
