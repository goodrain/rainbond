package vmrestore

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

type ledgerSQLTrace struct {
	mu     sync.Mutex
	writes []string
}

func (l *ledgerSQLTrace) Print(args ...interface{}) {
	if len(args) <= 3 {
		return
	}
	query, ok := args[3].(string)
	if !ok {
		return
	}
	upper := strings.ToUpper(strings.TrimSpace(query))
	if strings.HasPrefix(upper, "SELECT ") && !strings.Contains(upper, "FOR UPDATE") {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writes = append(l.writes, query)
}

// capability_id: rainbond.vm-restore.read-only-ledger-view
func TestStoreReadOnlyUsesSelectsAndPreservesWriterAdmission(t *testing.T) {
	for _, sealed := range []bool{false, true} {
		t.Run(fmt.Sprint(sealed), func(t *testing.T) {
			store, database, scope := fixtureStore(t)
			operationID := ""
			if sealed {
				e := expectation()
				if _, err := store.Mutate(scope, e, prepareState(scope, e)); err != nil {
					t.Fatal(err)
				}
				operationID = e.OperationID
			}
			before, err := store.Read(scope, operationID)
			if err != nil {
				t.Fatal(err)
			}
			database.DB().SetMaxOpenConns(1)
			if err := database.Exec("PRAGMA query_only=ON").Error; err != nil {
				t.Fatal(err)
			}
			trace := &ledgerSQLTrace{}
			database.SetLogger(trace)
			database.LogMode(true)
			after, err := store.ReadOnly(scope, operationID)
			if err != nil || !reflect.DeepEqual(before, after) || len(trace.writes) != 0 {
				t.Fatalf("read-only ledger changed validation or used writes: %v %v", err, trace.writes)
			}
			database.LogMode(false)
			if _, err := store.Read(scope, operationID); err == nil {
				t.Fatal("ordinary locked read lost its SQLite writer-lock path")
			}
			called := false
			if _, err := store.Mutate(scope, Expectation{}, func(c Current) (Mutation, error) {
				called = true
				return Mutation{State: c.State}, nil
			}); err == nil || called {
				t.Fatal("mutation bypassed writer admission on a read-only connection")
			}
		})
	}
}

// capability_id: rainbond.vm-restore.read-only-ledger-view
func TestStoreReadOnlySupportsConcurrentReadersOnReadOnlyDatabase(t *testing.T) {
	store, database, scope := fixtureStore(t)
	e := expectation()
	want, err := store.Mutate(scope, e, prepareState(scope, e))
	if err != nil {
		t.Fatal(err)
	}
	var sequence int
	var name, filename string
	if err := database.DB().QueryRow("PRAGMA database_list").Scan(&sequence, &name, &filename); err != nil {
		t.Fatal(err)
	}
	dsn := (&url.URL{Scheme: "file", Path: filename, RawQuery: "mode=ro"}).String()
	readonly, err := gorm.Open("sqlite3", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer readonly.Close()
	readonly.DB().SetMaxOpenConns(8)
	reader := NewStore(readonly)
	trace := &ledgerSQLTrace{}
	readonly.SetLogger(trace)
	readonly.LogMode(true)
	start, failures := make(chan struct{}), make(chan error, 8)
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for j := 0; j < 4; j++ {
				got, err := reader.ReadOnly(scope, e.OperationID)
				if err != nil || !reflect.DeepEqual(want, got) {
					failures <- fmt.Errorf("read-only concurrent ledger mismatch: %v", err)
					return
				}
			}
		}()
	}
	close(start)
	workers.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	if len(trace.writes) != 0 {
		t.Fatal("concurrent reads requested writer SQL", trace.writes)
	}
}

// capability_id: rainbond.vm-restore.read-only-ledger-view
func TestStoreReadOnlyKeepsStrictLedgerValidation(t *testing.T) {
	for _, change := range []string{"duplicate-state", "foreign-state", "bad-state", "missing-state", "missing-fence-receipt", "unapplied-receipt", "wrong-namespace", "wrong-tenant", "duplicate-service"} {
		t.Run(change, func(t *testing.T) {
			store, database, scope := fixtureStore(t)
			e := expectation()
			prepared, err := store.Mutate(scope, e, prepareState(scope, e))
			if err != nil {
				t.Fatal(err)
			}
			stateRow := database.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name = ?", scope.ServiceID, StateKey)
			switch change {
			case "duplicate-state":
				var row dbmodel.ComponentK8sAttributes
				err = stateRow.First(&row).Error
				row.ID = 0
				if err == nil {
					err = database.Create(&row).Error
				}
			case "foreign-state":
				err = stateRow.Update("tenant_id", "other").Error
			case "bad-state":
				err = stateRow.Update("attribute_value", "invalid-json").Error
			case "missing-state":
				err = stateRow.Delete(&dbmodel.ComponentK8sAttributes{}).Error
			case "missing-fence-receipt", "unapplied-receipt":
				key, _ := ReceiptKey(e.OperationID)
				row := database.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name = ?", scope.ServiceID, key)
				if change == "missing-fence-receipt" {
					err = row.Delete(&dbmodel.ComponentK8sAttributes{}).Error
				} else {
					step := prepared.Receipt.Steps[e.Step]
					step.Applied = false
					prepared.Receipt.Steps[e.Step] = step
					value, encodeErr := EncodeRecord(prepared.Receipt)
					if encodeErr != nil {
						t.Fatal(encodeErr)
					}
					err = row.Update("attribute_value", value).Error
				}
			case "wrong-namespace":
				scope.Namespace = "other"
			case "wrong-tenant":
				scope.TenantID = "other"
			case "duplicate-service":
				err = database.Create(&dbmodel.TenantServices{TenantID: scope.TenantID, ServiceID: scope.ServiceID, ExtendMethod: string(dbmodel.ServiceTypeVM)}).Error
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, operationID := range []string{"", e.OperationID} {
				if _, err := store.Read(scope, operationID); !errors.Is(err, ErrCorrupt) {
					t.Fatal("locked baseline accepted corrupt ledger", err)
				}
				if _, err := store.ReadOnly(scope, operationID); !errors.Is(err, ErrCorrupt) {
					t.Fatal("read-only view weakened ledger validation", err)
				}
			}
		})
	}
}
