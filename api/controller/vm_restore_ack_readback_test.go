package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// This adapter supplies authenticated scope and an optional response-loss fault.
// All commands and status DTOs come from the actual persistent Store and Engine.
type ackReadbackFacade struct {
	engine       *vmrestore.RestoreEngine
	scope        vmrestore.Scope
	unknownReply bool
}

func (a *ackReadbackFacade) authorize(ctx context.Context, service string) error {
	if service != a.scope.ServiceID || ctx.Value(ctxutil.ContextKey("tenant_id")) != a.scope.TenantID {
		return vmrestore.ErrNotFound
	}
	return nil
}
func (a *ackReadbackFacade) GetVMRestoreControl(ctx context.Context, service string) (vmrestore.RestoreControlDTO, error) {
	if err := a.authorize(ctx, service); err != nil {
		return vmrestore.RestoreControlDTO{}, err
	}
	return a.engine.Control(a.scope)
}
func (a *ackReadbackFacade) GetVMRestore(ctx context.Context, service, op string) (vmrestore.RestoreOperationDTO, error) {
	if err := a.authorize(ctx, service); err != nil {
		return vmrestore.RestoreOperationDTO{}, err
	}
	return a.engine.Status(a.scope, op)
}
func (a *ackReadbackFacade) VMRestoreCommand(ctx context.Context, service, op, action string, request vmrestore.RestoreCommand) (vmrestore.RestoreOperationDTO, error) {
	if err := a.authorize(ctx, service); err != nil {
		return vmrestore.RestoreOperationDTO{}, err
	}
	dto, err := a.engine.Command(ctx, a.scope, op, action, request)
	if err == nil && a.unknownReply {
		return dto, vmrestore.ErrRestoreUnknown
	}
	return dto, err
}

func ackReadbackServer(a *ackReadbackFacade) *httptest.Server {
	router := restoreHTTPRouter(&VMRestoreController{service: a})
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if scope := r.Header.Get("X-Test-Scope"); scope != "" {
			tenant := a.scope.TenantID
			if scope == "foreign" {
				tenant = "foreign-tenant"
			}
			ctx := context.WithValue(r.Context(), ctxutil.ContextKey("tenant_id"), tenant)
			ctx = context.WithValue(ctx, ctxutil.ContextKey("service_id"), a.scope.ServiceID)
			r = r.WithContext(ctx)
		}
		router.ServeHTTP(w, r)
	}))
}

func ackReadbackHTTP(t *testing.T, server *httptest.Server, method, path, scope string, body []byte, discard bool) (int, map[string]interface{}) {
	t.Helper()
	r, err := http.NewRequest(method, server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("X-Test-Scope", scope)
	r.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if discard {
		return response.StatusCode, nil
	}
	var envelope struct {
		Bean map[string]interface{} `json:"bean"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, envelope.Bean
}

func ackReadbackDatabase(t *testing.T, path string) *gorm.DB {
	t.Helper()
	database, err := gorm.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	database.LogMode(false)
	return database
}

func seedAckReadbackCommit(t *testing.T, database *gorm.DB, scope vmrestore.Scope, rollback bool) *vmrestore.Store {
	t.Helper()
	if err := database.AutoMigrate(&dbmodel.Tenants{}, &dbmodel.TenantServices{}, &dbmodel.ComponentK8sAttributes{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&dbmodel.Tenants{UUID: scope.TenantID, Namespace: scope.Namespace}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&dbmodel.TenantServices{TenantID: scope.TenantID, ServiceID: scope.ServiceID, ExtendMethod: string(dbmodel.ServiceTypeVM)}).Error; err != nil {
		t.Fatal(err)
	}
	store := vmrestore.NewStore(database)
	// The fixture supplies an already committed Region precondition, not native
	// copy/storage qualification. ACK acceptance below uses the actual command.
	config := vmrestore.RestoreConfig{Hardware: vmrestore.SnapshotConfig{CPUMilli: 2000, MemoryMiB: 2048, BootMode: "bios", DiskLayout: []vmrestore.SnapshotLayout{{VolumeID: 7, DiskKey: "disk", CanonicalName: "manual7", Role: "root", Device: "disk", Bus: "virtio", BootOrder: 1}}}, Volumes: []vmrestore.RestoreVolumeConfig{{VolumeID: 7, DiskKey: "disk", CapacityGiB: 8, StorageClass: "cephfs-vm", AccessMode: "ReadWriteMany"}}}
	hash, err := vmrestore.RestoreConfigHash(config)
	if err != nil {
		t.Fatal(err)
	}
	intent := vmrestore.RestoreIntent{CurrentVM: vmrestore.ObjectRef{Kind: "VirtualMachine", Namespace: scope.Namespace, Name: "vm", UID: "target-vm"}, SourceSnapshot: vmrestore.ObjectRef{Kind: "VirtualMachineSnapshot", Namespace: scope.Namespace, Name: "snapshot", UID: "snapshot-uid"}, SourceContent: vmrestore.ObjectRef{Kind: "VirtualMachineSnapshotContent", Namespace: scope.Namespace, Name: "content", UID: "content-uid"}, SourceVMUID: "source-vm", SourceCoreHash: strings.Repeat("a", 64), SourceDocumentHash: strings.Repeat("c", 64), Before: config, ConfigAfterHash: hash, PowerAfter: "stopped"}
	expect := vmrestore.Expectation{OperationID: "ack-op", ManifestHash: strings.Repeat("a", 64), RequestHash: strings.Repeat("b", 64), Step: "restore-commit"}
	current, err := store.Mutate(scope, expect, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		r := vmrestore.NewReceipt(scope, expect.OperationID, expect.ManifestHash)
		r.Restore, r.ConfigCommitHash = &intent, strings.Repeat("b", 64)
		c.State.Fence = &vmrestore.Fence{Kind: "restore", Phase: "region-committed", OperationID: expect.OperationID, ManifestHash: expect.ManifestHash, TargetVMUID: intent.CurrentVM.UID}
		return vmrestore.Mutation{State: c.State, Receipt: &r}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if rollback {
		ack := expect
		ack.ControlRevision = current.State.ControlRevision
		ack.Step = "restore-console-ack"
		current, err = store.AcknowledgeRestoreDB(scope, ack, current.Receipt.ConfigCommitHash, strings.Repeat("9", 64))
		if err != nil {
			t.Fatal(err)
		}
		expect.ControlRevision = current.State.ControlRevision
		expect.Step = "restore-rollback"
		_, err = store.Mutate(scope, expect, func(c vmrestore.Current) (vmrestore.Mutation, error) {
			c.State.Fence.Phase = "region-rolled-back"
			c.Receipt.RollbackConfigHash = strings.Repeat("f", 64)
			return vmrestore.Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return store
}

// capability_id: rainbond.vm-restore.accepted-ack-readback
func TestRestoreAcceptedAckReadbackAfterLostReplyAndReopen(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		for _, unknown := range []bool{false, true} {
			action, field, acked := "config-ack", "console_ack_hash", "console_acked"
			if rollback {
				action, field, acked = "rollback-ack", "rollback_ack_hash", "rollback_acked"
			}
			name := action + "/discarded-success"
			if unknown {
				name = action + "/unknown-after-accepted-commit"
			}
			t.Run(name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "ack.db")
				database := ackReadbackDatabase(t, path)
				scope := vmrestore.Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
				store := seedAckReadbackCommit(t, database, scope, rollback)
				facade := &ackReadbackFacade{engine: &vmrestore.RestoreEngine{Store: store}, scope: scope, unknownReply: unknown}
				server := ackReadbackServer(facade)
				t.Cleanup(func() { server.Close(); database.Close() })
				dto, err := facade.engine.Status(scope, "ack-op")
				if err != nil {
					t.Fatal(err)
				}
				_, unacked := ackReadbackHTTP(t, server, "GET", "/vm-restores/ack-op", "self", nil, false)
				if unacked[acked] != false || (unacked[field] != nil && unacked[field] != "") {
					t.Fatal("unaccepted request acquired ACK evidence", unacked)
				}
				commit := dto.ConfigCommitHash
				if rollback {
					commit = dto.RollbackConfigHash
				}
				document := strings.Repeat("e", 64)
				request := vmrestore.RestoreCommand{ExpectedOperationEpoch: dto.OperationEpoch, ExpectedControlRevision: dto.ControlRevision, ExpectedBindingGeneration: dto.BindingGeneration, CommitHash: commit, DocumentHash: document}
				body, _ := json.Marshal(request)
				status, reply := ackReadbackHTTP(t, server, "POST", "/vm-restores/ack-op/steps/"+action, "self", body, !unknown)
				if status != 202 || (unknown && (reply["error_code"] != "vm_restore_outcome_unknown" || reply[field] != nil || reply["complete"] != false)) {
					t.Fatal("unknown response became success evidence", status, reply)
				}
				server.Close()
				if err := database.Close(); err != nil {
					t.Fatal(err)
				}
				database = ackReadbackDatabase(t, path)
				store = vmrestore.NewStore(database)
				facade = &ackReadbackFacade{engine: &vmrestore.RestoreEngine{Store: store}, scope: scope}
				server = ackReadbackServer(facade)
				before, err := store.Read(scope, "ack-op")
				if err != nil {
					t.Fatal(err)
				}
				persisted := before.Receipt.ConsoleAckHash
				if rollback {
					persisted = before.Receipt.RollbackAckHash
				}
				if persisted != document {
					t.Fatal("actual ACK command did not durably accept document")
				}
				status, readback := ackReadbackHTTP(t, server, "GET", "/vm-restores/ack-op?document_hash="+strings.Repeat("d", 64), "self", nil, false)
				if status != 200 || readback[acked] != true || readback["operation_epoch"] != dto.OperationEpoch {
					t.Fatal("persisted ACK identity disappeared", status, readback)
				}
				if readback[field] != persisted {
					t.Errorf("GET cannot independently match the accepted document: %s=%v, persisted=%s", field, readback[field], persisted)
				}
				if rollback && readback["console_ack_hash"] != before.Receipt.ConsoleAckHash {
					t.Error("rollback readback lost the separate original restore ACK")
				}
				if readback["source"].(map[string]interface{})["document_hash"] == document {
					t.Fatal("fixture confused snapshot source document with accepted restore document")
				}
				after, _ := store.Read(scope, "ack-op")
				if !reflect.DeepEqual(before, after) {
					t.Fatal("GET changed durable ledger")
				}
				status, replay := ackReadbackHTTP(t, server, "POST", "/vm-restores/ack-op/steps/"+action, "self", body, false)
				if status != 202 || replay["applied"] != true || replay[acked] != true {
					t.Fatal("exact old-CAS request could not reconcile lost response", status, replay)
				}
				if replay[field] != persisted {
					t.Errorf("successful POST omits accepted document: %s=%v", field, replay[field])
				}
				after, _ = store.Read(scope, "ack-op")
				if !reflect.DeepEqual(before, after) {
					t.Fatal("exact retry repeated durable ACK or claim")
				}
				request.DocumentHash = strings.Repeat("d", 64)
				body, _ = json.Marshal(request)
				status, conflict := ackReadbackHTTP(t, server, "POST", "/vm-restores/ack-op/steps/"+action, "self", body, false)
				if status != 409 || conflict[field] != nil {
					t.Fatal("old-CAS different document acquired success evidence", status, conflict)
				}
				after, _ = store.Read(scope, "ack-op")
				if !reflect.DeepEqual(before, after) {
					t.Fatal("old-CAS conflicting request changed authority")
				}
				request.ExpectedControlRevision = after.State.ControlRevision
				request.ExpectedBindingGeneration = after.State.BindingGeneration
				body, _ = json.Marshal(request)
				status, conflict = ackReadbackHTTP(t, server, "POST", "/vm-restores/ack-op/steps/"+action, "self", body, false)
				if status != 409 || conflict[field] != nil {
					t.Fatal("fresh-CAS conflicting document became accepted evidence", status, conflict)
				}
				after, _ = store.Read(scope, "ack-op")
				if after.State.ControlRevision != before.State.ControlRevision+1 || after.Receipt.ConsoleAckHash != before.Receipt.ConsoleAckHash || after.Receipt.RollbackAckHash != before.Receipt.RollbackAckHash {
					t.Fatal("fresh-CAS conflict did not preserve ACK while recording its RPC claim")
				}
				for _, context := range []struct {
					scope  string
					status int
				}{{"", 403}, {"foreign", 404}} {
					status, denied := ackReadbackHTTP(t, server, "GET", "/vm-restores/ack-op", context.scope, nil, false)
					if status != context.status || denied[field] != nil {
						t.Fatal("ACK evidence escaped authenticated scope", status, denied)
					}
				}
			})
		}
	}
}
