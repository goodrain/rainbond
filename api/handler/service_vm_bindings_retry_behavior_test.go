package handler

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	apimodel "github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
	"github.com/mattn/go-sqlite3"
)

var configRetryDriverSequence atomic.Uint64

type configRetrySQLDriver struct {
	driver.Driver
	beforeBegin func() error
}

func (d configRetrySQLDriver) Open(name string) (driver.Conn, error) {
	connection, err := d.Driver.Open(name)
	if err != nil {
		return nil, err
	}
	return configRetrySQLConnection{Conn: connection, beforeBegin: d.beforeBegin}, nil
}

type configRetrySQLConnection struct {
	driver.Conn
	beforeBegin func() error
}

func (c configRetrySQLConnection) Begin() (driver.Tx, error) {
	if err := c.beforeBegin(); err != nil {
		return nil, err
	}
	return c.Conn.Begin()
}

func configRetryFixture(t *testing.T) bindingConsumerFixture {
	t.Helper()
	f := newBindingConsumerFixture(t)
	f.action.dbmanager = independentRealAddManager{bindingConsumerDB{database: f.database}}
	current, _ := f.store.Read(f.scope, "")
	_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		c.State.Bindings = map[string]vmrestore.Binding{}
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func configRetryRequest(f bindingConsumerFixture) *dbmodel.TenantServiceVolume {
	return &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: "receipt-config",
		VolumePath: "/etc/receipt-config", VolumeType: dbmodel.ConfigFileVolumeType.String()}
}

// capability_id: rainbond.vm-disk.active-binding-retry-repair
func TestConfigRetryRealSQLFailuresRetainOnlyUncertainOwnership(t *testing.T) {
	for _, phase := range []string{"volume", "config", "checkpoint", "commit", "complete"} {
		t.Run(phase, func(t *testing.T) {
			f := configRetryFixture(t)
			f.database.DB().SetMaxOpenConns(1)
			statements := []string{}
			switch phase {
			case "volume":
				statements = append(statements, "CREATE TRIGGER retry_guard BEFORE INSERT ON tenant_services_volume WHEN NEW.volume_name = 'receipt-config' BEGIN SELECT RAISE(ABORT, 'fixture volume failure'); END")
			case "config":
				statements = append(statements, "CREATE TRIGGER retry_guard BEFORE INSERT ON tenant_service_config_file BEGIN SELECT RAISE(ABORT, 'fixture config failure'); END")
			case "checkpoint", "complete":
				step := "config-sql"
				if phase == "complete" {
					step = "complete"
				}
				statements = append(statements, "CREATE TRIGGER retry_guard BEFORE UPDATE OF attribute_value ON component_k8s_attributes WHEN NEW.name LIKE 'vm_restore_receipt_%' AND NEW.attribute_value LIKE '%"+step+"%' BEGIN SELECT RAISE(ABORT, 'fixture receipt failure'); END")
			case "commit":
				statements = append(statements, "PRAGMA foreign_keys = ON",
					"CREATE TABLE allowed_config_identity (id INTEGER PRIMARY KEY)",
					"CREATE TABLE config_commit_guard (volume_id INTEGER REFERENCES allowed_config_identity(id) DEFERRABLE INITIALLY DEFERRED)",
					"CREATE TRIGGER retry_guard AFTER INSERT ON tenant_services_volume WHEN NEW.volume_name = 'receipt-config' BEGIN INSERT INTO config_commit_guard(volume_id) VALUES (NEW.ID); END")
			}
			for _, statement := range statements {
				if err := f.database.Exec(statement).Error; err != nil {
					t.Fatal(err)
				}
			}
			syncs := 0
			f.action.syncVirtualMachineSpecHook = func(string) error { syncs++; return nil }
			if failure := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "fixture-only", "add"); failure == nil {
				t.Fatal("expected actual SQL failure")
			}
			current, err := f.store.Read(f.scope, "")
			if err != nil {
				t.Fatal(err)
			}
			uncertain := phase == "commit" || phase == "complete"
			if uncertain {
				if current.State.Admission == nil || !current.State.Admission.OutcomeUnknown {
					t.Errorf("uncertain SQL/sync outcome lost a retryable owner: phase=%s", phase)
				}
			} else if current.State.Admission != nil {
				t.Errorf("known SQL rollback retained admission: phase=%s", phase)
			}
			var volumes, configs int
			f.database.Model(&dbmodel.TenantServiceVolume{}).Where("volume_name = ?", "receipt-config").Count(&volumes)
			f.database.Model(&dbmodel.TenantServiceConfigFile{}).Where("volume_name = ?", "receipt-config").Count(&configs)
			want := 0
			if phase == "complete" {
				want = 1
			}
			if volumes != want || configs != want || syncs != want {
				t.Fatalf("SQL boundary was not atomic: phase=%s volumes=%d configs=%d syncs=%d", phase, volumes, configs, syncs)
			}
			if err := f.database.Exec("DROP TRIGGER retry_guard").Error; err != nil {
				t.Fatal(err)
			}
			if failure := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "fixture-only", "add"); failure != nil {
				t.Fatalf("actual SQL recovery could not complete: %v", failure)
			}
			current, err = f.store.Read(f.scope, "")
			if err != nil || current.State.Admission != nil {
				t.Fatal("successful SQL recovery retained admission")
			}
			f.database.Model(&dbmodel.TenantServiceVolume{}).Where("volume_name = ?", "receipt-config").Count(&volumes)
			f.database.Model(&dbmodel.TenantServiceConfigFile{}).Where("volume_name = ?", "receipt-config").Count(&configs)
			if volumes != 1 || configs != 1 {
				t.Fatal("SQL recovery duplicated configuration")
			}
		})
	}
}

// capability_id: rainbond.vm-disk.active-binding-retry-repair
func TestConfigRetryRejectsDifferentPayloadAndReplacedSQLIdentity(t *testing.T) {
	for _, scenario := range []string{"content", "path", "mode", "name", "volume-id", "config-id", "stored-content", "generation", "foreign-owner"} {
		t.Run(scenario, func(t *testing.T) {
			f := configRetryFixture(t)
			syncs := 0
			f.action.syncVirtualMachineSpecHook = func(string) error { syncs++; return fmt.Errorf("fixture unknown sync") }
			if err := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "fixture-only", "add"); err == nil {
				t.Fatal("expected committed SQL and unknown sync")
			}
			before, _ := f.store.Read(f.scope, "")
			owner := before.State.Admission.Token
			request, content := configRetryRequest(f), "fixture-only"
			switch scenario {
			case "content":
				content = "different-fixture"
			case "path":
				request.VolumePath = "/etc/different"
			case "mode":
				mode := int32(0600)
				request.Mode = &mode
			case "name":
				request.VolumeName = "other-config"
			case "volume-id":
				if err := f.database.Exec("UPDATE tenant_services_volume SET ID = 999 WHERE volume_name = 'receipt-config'").Error; err != nil {
					t.Fatal(err)
				}
			case "config-id":
				if err := f.database.Exec("UPDATE tenant_service_config_file SET ID = 999 WHERE volume_name = 'receipt-config'").Error; err != nil {
					t.Fatal(err)
				}
			case "stored-content":
				if err := f.database.Exec("UPDATE tenant_service_config_file SET file_content = 'different-fixture' WHERE volume_name = 'receipt-config'").Error; err != nil {
					t.Fatal(err)
				}
			case "generation", "foreign-owner":
				_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: before.State.ControlRevision, BindingGeneration: before.State.BindingGeneration, AdmissionOwner: owner}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
					if scenario == "generation" {
						c.State.BindingGeneration++
					} else {
						if err := c.State.FinishMutation(owner, true); err != nil {
							return vmrestore.Mutation{}, err
						}
						if err := c.State.AdmitMutation("foreign-owner", "different-action"); err != nil {
							return vmrestore.Mutation{}, err
						}
					}
					return vmrestore.Mutation{State: c.State}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if failure := f.action.VolumnVar(request, f.scope.TenantID, content, "add"); failure == nil || failure.Code != 409 {
				t.Fatalf("different request/identity was treated as original retry: %v", failure)
			}
			current, err := f.store.Read(f.scope, "")
			if err != nil || current.State.Admission == nil || syncs != 1 {
				t.Fatal("refused retry lost admission or repeated sync")
			}
			if scenario == "foreign-owner" {
				if current.State.Admission.Token != "foreign-owner" || current.State.Admission.OutcomeUnknown {
					t.Fatal("foreign admission was changed")
				}
			} else if current.State.Admission.Token != owner || !current.State.Admission.OutcomeUnknown {
				t.Fatal("original unknown owner was cleared or left executing")
			}
		})
	}
}

// capability_id: rainbond.vm-disk.active-binding-retry-repair
func TestConfigRetryDoesNotPermitOrdinaryDuplicateButAllowsRecreation(t *testing.T) {
	f := configRetryFixture(t)
	syncs := 0
	f.action.syncVirtualMachineSpecHook = func(string) error { syncs++; return nil }
	if failure := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "fixture-only", "add"); failure != nil {
		t.Fatal(failure)
	}
	if failure := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "fixture-only", "add"); failure == nil {
		t.Fatal("ordinary duplicate bypassed the real Volume DAO")
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil || current.State.Admission != nil || syncs != 1 {
		t.Fatal("ordinary duplicate changed sync or retained a new admission")
	}
	if failure := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "", "delete"); failure != nil {
		t.Fatal(failure)
	}
	request := configRetryRequest(f)
	request.VolumePath = "/etc/recreated"
	if failure := f.action.VolumnVar(request, f.scope.TenantID, "new-fixture", "add"); failure != nil {
		t.Fatalf("completed historical receipt blocked legitimate recreation: %v", failure)
	}
}

// capability_id: rainbond.vm-disk.active-binding-retry-repair
func TestConfigRetryReceiptsDoNotBlockOrdinaryVolumeResolution(t *testing.T) {
	f := configRetryFixture(t)
	syncs := 0
	f.action.syncVirtualMachineSpecHook = func(string) error {
		syncs++
		if syncs == 1 {
			return fmt.Errorf("fixture unknown sync")
		}
		return nil
	}
	if failure := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "fixture-only", "add"); failure == nil {
		t.Fatal("expected first unknown sync")
	}
	if failure := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "fixture-only", "add"); failure != nil {
		t.Fatal(failure)
	}
	request := &apimodel.UpdVolumeReq{VolumeName: f.volume.VolumeName, VolumePath: f.volume.VolumePath,
		VolumeType: f.volume.VolumeType, VolumeCapacity: int64PtrForBinding(2)}
	if err := f.action.UpdVolume(f.scope.ServiceID, request); err != nil {
		t.Fatalf("completed config receipt corrupted the ordinary shared resolver: %v", err)
	}
}

// capability_id: rainbond.vm-disk.active-binding-retry-repair
func TestConfigRetryClaimExcludesConcurrentSyncUsingRealSQL(t *testing.T) {
	f := configRetryFixture(t)
	var syncs atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	f.action.syncVirtualMachineSpecHook = func(string) error {
		switch syncs.Add(1) {
		case 1:
			return fmt.Errorf("fixture unknown sync")
		case 2:
			close(entered)
			<-release
		}
		return nil
	}
	if err := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "fixture-only", "add"); err == nil {
		t.Fatal("expected first unknown sync")
	}
	done := make(chan error, 1)
	go func() {
		if failure := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "fixture-only", "add"); failure != nil {
			done <- failure
		} else {
			done <- nil
		}
	}()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("retry did not reach sync: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("retry did not reach sync")
	}
	duplicate := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "fixture-only", "add")
	current, err := f.store.Read(f.scope, "")
	close(release)
	first := <-done
	if duplicate == nil || duplicate.Code != 409 || syncs.Load() != 2 || first != nil {
		t.Fatalf("concurrent duplicate entered sync: failure=%v syncs=%d first=%v", duplicate, syncs.Load(), first)
	}
	if err != nil || current.State.Admission == nil || current.State.Admission.OutcomeUnknown {
		t.Fatal("executing sync retry lost its claimed owner")
	}
	current, err = f.store.Read(f.scope, "")
	if err != nil || current.State.Admission != nil {
		t.Fatal("completed sync retry retained admission")
	}
}

// capability_id: rainbond.vm-disk.active-binding-retry-repair
func TestConfigRetryRevalidatesStateChangedBetweenActualSQLReads(t *testing.T) {
	for _, scenario := range []string{"generation", "binding"} {
		t.Run(scenario, func(t *testing.T) {
			f := configRetryFixture(t)
			syncs := 0
			f.action.syncVirtualMachineSpecHook = func(string) error { syncs++; return fmt.Errorf("fixture unknown sync") }
			if failure := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "fixture-only", "add"); failure == nil {
				t.Fatal("expected committed config with unknown sync")
			}
			before, _ := f.store.Read(f.scope, "")
			var databases []struct{ File string }
			if err := f.database.Raw("PRAGMA database_list").Scan(&databases).Error; err != nil || len(databases) != 1 {
				t.Fatal("missing actual private SQLite path")
			}
			owner := before.State.Admission.Token
			var begins atomic.Int32
			changed := false
			driverName := fmt.Sprintf("config-retry-real-sql-%d", configRetryDriverSequence.Add(1))
			sql.Register(driverName, configRetrySQLDriver{Driver: &sqlite3.SQLiteDriver{}, beforeBegin: func() error {
				if begins.Add(1) != 2 {
					return nil
				}
				_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: before.State.ControlRevision, BindingGeneration: before.State.BindingGeneration, AdmissionOwner: owner}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
					c.State.BindingGeneration++
					if scenario == "binding" {
						c.State.Bindings["42"] = vmrestore.Binding{VolumeID: 42, DiskKey: "data-1", CanonicalName: "manual42", Role: "data", Attached: true,
							ActivePVC: vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: "active-a", UID: "a-uid"}}
					}
					return vmrestore.Mutation{State: c.State}, nil
				})
				changed = err == nil
				return err
			}})
			connection, err := sql.Open(driverName, databases[0].File)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { connection.Close() })
			database, err := gorm.Open("sqlite3", connection)
			if err != nil {
				t.Fatal(err)
			}
			f.action.dbmanager = independentRealAddManager{bindingConsumerDB{database: database}}
			failure := f.action.VolumnVar(configRetryRequest(f), f.scope.TenantID, "fixture-only", "add")
			current, err := f.store.Read(f.scope, "")
			if !changed || err != nil || current.State.BindingGeneration != before.State.BindingGeneration+1 {
				t.Fatal("actual interleaved state mutation was not executed")
			}
			if failure == nil || failure.Code != 409 || syncs != 1 || current.State.Admission == nil || !current.State.Admission.OutcomeUnknown || current.State.Admission.Token != before.State.Admission.Token {
				t.Fatalf("retry used stale first-read state: failure=%v syncs=%d", failure, syncs)
			}
		})
	}
}
