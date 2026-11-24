package vmrestore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbmodel "github.com/goodrain/rainbond/db/model"
	csi "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kv "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
)

type fakeSnapshotBackend struct {
	mu                                            sync.Mutex
	contents                                      map[string]*csi.VolumeSnapshotContent
	resumeEntered                                 chan struct{}
	resumeRelease                                 chan struct{}
	resumeError                                   error
	haltError                                     error
	haltAcceptedOnly                              bool
	contextError                                  error
	observeError                                  error
	deleteError                                   error
	fixture                                       snapshotFixture
	stopped, running, unlocked, deleted, residual bool
	creates, halts, resumes, deletes              int
	createError                                   error
}

func (b *fakeSnapshotBackend) Context(context.Context, Scope) (SnapshotContext, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.contextError != nil {
		return SnapshotContext{}, b.contextError
	}
	source := b.fixture.content.Spec.Source.VirtualMachine
	config, err := SnapshotSafeConfig(nativeAsVM(source), b.fixture.intent.Disks)
	return SnapshotContext{SourceVM: refOf("VirtualMachine", source), WasRunning: b.running, Stopped: b.stopped, Config: config, Disks: b.fixture.intent.Disks}, err
}
func (b *fakeSnapshotBackend) Exists(context.Context, Scope, string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.creates > 0 && !b.deleted, nil
}
func (b *fakeSnapshotBackend) Observe(_ context.Context, _ Scope, _ SnapshotIntent, _ []ResourceRef) (SnapshotObservation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.observeError != nil {
		return SnapshotObservation{}, b.observeError
	}
	observed := SnapshotObservation{SourceStopped: b.stopped, SourceRunning: b.running, SourceLockClear: b.unlocked, CleanupComplete: (b.creates == 0 || b.deleted) && !b.residual, Volumes: b.fixture.volumes, Contents: b.contents}
	if b.creates > 0 && !b.deleted {
		observed.Snapshot = b.fixture.snapshot
		observed.Content = b.fixture.content
		observed.Resources = []ResourceRef{{ObjectRef: refOf("VirtualMachineSnapshot", b.fixture.snapshot), Purpose: "snapshot"}, {ObjectRef: refOf("VirtualMachineSnapshotContent", b.fixture.content), Purpose: "snapshot-content"}}
		for _, volume := range b.fixture.volumes {
			observed.Resources = append(observed.Resources, ResourceRef{ObjectRef: refOf("VolumeSnapshot", volume), Purpose: "volume-snapshot"})
		}
		for _, content := range b.contents {
			observed.Resources = append(observed.Resources, ResourceRef{ObjectRef: refOf("VolumeSnapshotContent", content), Purpose: "csi-snapshot-content"})
		}
	}
	return observed, nil
}
func (b *fakeSnapshotBackend) Halt(context.Context, Scope, SnapshotIntent) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.halts++
	if b.haltError != nil {
		return b.haltError
	}
	if b.haltAcceptedOnly {
		return nil
	}
	b.stopped = true
	b.running = false
	return nil
}
func (b *fakeSnapshotBackend) Create(context.Context, Scope, SnapshotIntent, string, string) (ObjectRef, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.createError != nil {
		return ObjectRef{}, b.createError
	}
	b.creates++
	return refOf("VirtualMachineSnapshot", b.fixture.snapshot), nil
}
func (b *fakeSnapshotBackend) Resume(context.Context, Scope, SnapshotIntent) error {
	b.mu.Lock()
	b.resumes++
	entered, release, resumeErr := b.resumeEntered, b.resumeRelease, b.resumeError
	b.mu.Unlock()
	if entered != nil {
		entered <- struct{}{}
	}
	if release != nil {
		<-release
	}
	if resumeErr != nil {
		return resumeErr
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped = false
	b.running = true
	return nil
}
func (b *fakeSnapshotBackend) Delete(context.Context, Scope, []ResourceRef) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deletes++
	if b.deleteError != nil {
		return b.deleteError
	}
	b.deleted = true
	return nil
}
func snapshotEngineFixture(t *testing.T) (*SnapshotEngine, *fakeSnapshotBackend, Scope) {
	t.Helper()
	store, _, scope := fixtureStore(t)
	if err := store.db.AutoMigrate(&dbmodel.ServiceEvent{}).Error; err != nil {
		t.Fatal(err)
	}
	fixture := snapshotProjectionFixture(t)
	backend := &fakeSnapshotBackend{fixture: fixture, running: true, unlocked: true}
	setFakeSnapshotContents(backend)
	return NewSnapshotEngine(store, backend), backend, scope
}
func enginePrepare(t *testing.T, engine *SnapshotEngine, scope Scope, backend *fakeSnapshotBackend) SnapshotOperationDTO {
	t.Helper()
	result, err := engine.Prepare(context.Background(), scope, SnapshotCreateRequest{Name: backend.fixture.intent.Name, Description: backend.fixture.intent.Description, RequestID: backend.fixture.op, ManifestID: "manifest-a", PowerAfter: "previous"})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func engineAction(phase string, prepared SnapshotOperationDTO) SnapshotActionRequest {
	return SnapshotActionRequest{OperationID: prepared.OperationID, CoreHash: prepared.CoreHash, RequestID: phase, ManifestID: "manifest-a"}
}
func engineReady(t *testing.T, engine *SnapshotEngine, scope Scope, backend *fakeSnapshotBackend) SnapshotOperationDTO {
	t.Helper()
	prepared := enginePrepare(t, engine, scope, backend)
	if _, err := engine.Halt(context.Background(), scope, engineAction("halt", prepared)); err != nil {
		t.Fatal(err)
	}
	ready, err := engine.Create(context.Background(), scope, engineAction("create", prepared))
	if err != nil {
		t.Fatal(err)
	}
	return ready
}
func readyACK(phase string, ready SnapshotOperationDTO) SnapshotActionRequest {
	request := engineAction(phase, ready)
	request.SnapshotUID = ready.SnapshotUID
	request.ContentUID = ready.ContentUID
	request.NativeProofHash = ready.NativeProofHash
	request.AckKind = "ready"
	request.DocumentHash = strings.Repeat("d", 64)
	return request
}

// capability_id: rainbond.vm-snapshot.bounded-lifecycle
func TestRunningSnapshotWorkflowKeepsFenceUntilACKAndObservedPower(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	if !ready.ReadyToUse || !ready.SourceStopped || ready.ManifestACKed || backend.halts != 1 {
		t.Fatal("running workflow did not stop before native snapshot")
	}
	before, err := engine.store.Read(scope, ready.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Get(context.Background(), scope, ready.OperationID); err != nil {
		t.Fatal(err)
	}
	after, err := engine.store.Read(scope, ready.OperationID)
	if err != nil || after.State.ControlRevision != before.State.ControlRevision || after.State.Fence == nil {
		t.Fatal("GET auto-ACKed or released fence")
	}
	if _, err := engine.Finalize(context.Background(), scope, readyACK("finalize", ready)); !errors.Is(err, ErrSnapshotPending) {
		t.Fatal("finalize reported previous-running success without resuming VM")
	}
	resumed, err := engine.Resume(context.Background(), scope, readyACK("resume", ready))
	if err != nil || !resumed.SourceRunning || backend.resumes != 1 {
		t.Fatal("ACKed snapshot did not resume original running intent")
	}
	final, err := engine.Finalize(context.Background(), scope, readyACK("finalize", ready))
	if err != nil || !final.Finalized {
		t.Fatal("valid snapshot did not finalize")
	}
	current, err := engine.store.Read(scope, ready.OperationID)
	if err != nil || current.State.Fence != nil {
		t.Fatal("finalized snapshot retained fence")
	}
}

func TestWrongUIDOrIncompleteManifestCannotResume(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	request := readyACK("resume", ready)
	request.SnapshotUID = "wrong-uid"
	if _, err := engine.Resume(context.Background(), scope, request); err == nil || backend.resumes != 0 {
		t.Fatal("incorrect native identity was treated as durable manifest ACK")
	}
	request = readyACK("resume", ready)
	request.DocumentHash = ""
	if _, err := engine.Resume(context.Background(), scope, request); err == nil || backend.resumes != 0 {
		t.Fatal("missing manifest ACK allowed VM start")
	}
}

func TestNativeCreateRequiresStoppedAndRetriesDoNotRepeatCreate(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	prepared := enginePrepare(t, engine, scope, backend)
	if _, err := engine.Create(context.Background(), scope, engineAction("create", prepared)); !errors.Is(err, ErrSnapshotPending) || backend.creates != 0 {
		t.Fatal("native snapshot was created while source still running")
	}
	if _, err := engine.Halt(context.Background(), scope, engineAction("halt", prepared)); err != nil {
		t.Fatal(err)
	}
	request := engineAction("create", prepared)
	if _, err := engine.Create(context.Background(), scope, request); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Create(context.Background(), scope, request); err != nil || backend.creates != 1 {
		t.Fatal("same create step repeated native side effect")
	}
}

func TestCancelWaitsForResidualCleanupAndSourceUnlock(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	backend.residual = true
	backend.unlocked = false
	request := engineAction("cancel", ready)
	if _, err := engine.Cancel(context.Background(), scope, request); err != nil {
		t.Fatal(err)
	}
	current, err := engine.store.Read(scope, ready.OperationID)
	if err != nil || current.State.Fence == nil {
		t.Fatal("deletion request was mistaken for cleanup completion")
	}
	backend.residual = false
	if _, err := engine.Cancel(context.Background(), scope, request); !errors.Is(err, ErrSnapshotPending) {
		t.Fatal("source still locked but cancellation was completed")
	}
	backend.unlocked = true
	cancelled, err := engine.Cancel(context.Background(), scope, request)
	if err != nil || !cancelled.Cancelled {
		t.Fatal("unlocked complete cleanup did not become cancelled")
	}
	request = engineAction("resume-failure", ready)
	request.AckKind = "cancelled"
	request.DocumentHash = strings.Repeat("e", 64)
	if _, err := engine.Resume(context.Background(), scope, request); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "finalize-failure"
	if _, err := engine.Finalize(context.Background(), scope, request); err != nil {
		t.Fatal(err)
	}
	current, err = engine.store.Read(scope, ready.OperationID)
	if err != nil || current.State.Fence != nil {
		t.Fatal("failure cleanup did not restore power/release after failure ACK")
	}
}

func TestUnknownNativeCreateCannotBeCancelledAsIfNothingHappened(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	prepared := enginePrepare(t, engine, scope, backend)
	if _, err := engine.Halt(context.Background(), scope, engineAction("halt", prepared)); err != nil {
		t.Fatal(err)
	}
	backend.createError = ErrReferenceUnavailable
	if _, err := engine.Create(context.Background(), scope, engineAction("create", prepared)); err == nil {
		t.Fatal("expected transport outcome unknown")
	}
	if _, err := engine.Cancel(context.Background(), scope, engineAction("cancel", prepared)); !errors.Is(err, ErrBusy) {
		t.Fatal("unobserved in-flight create was released by cancel")
	}
	current, err := engine.store.Read(scope, prepared.OperationID)
	if err != nil || current.State.Fence == nil {
		t.Fatal("unknown create lost durable fence")
	}
}

func nativeAsVM(source *native.VirtualMachine) *kv.VirtualMachine {
	return &kv.VirtualMachine{ObjectMeta: source.ObjectMeta, Spec: source.Spec}
}

func (b *fakeSnapshotBackend) DeleteReferences(context.Context, Scope, []ResourceRef) error {
	return nil
}

func TestManagedSnapshotDeleteWaitsForAllChildrenAndPreservesSource(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	if _, err := engine.Resume(context.Background(), scope, readyACK("resume", ready)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Finalize(context.Background(), scope, readyACK("finalize", ready)); err != nil {
		t.Fatal(err)
	}
	request := engineAction("delete", ready)
	request.SnapshotUID = ready.SnapshotUID
	backend.residual = true
	if _, err := engine.Delete(context.Background(), scope, request); err != nil {
		t.Fatal(err)
	}
	current, err := engine.store.Read(scope, ready.OperationID)
	if err != nil || current.State.Fence == nil {
		t.Fatal("parent deletion completed while CSI residual remained")
	}
	backend.residual = false
	deleted, err := engine.Delete(context.Background(), scope, request)
	if err != nil || !deleted.Deleted {
		t.Fatal("fully reclaimed snapshot did not become deleted")
	}
	if !backend.running || backend.halts != 1 {
		t.Fatal("deleting ready snapshot changed source power")
	}
	list, err := engine.List(context.Background(), scope, 1, 20)
	if err != nil || list.Total != 0 {
		t.Fatal("deleted snapshot remained active in list")
	}
}

func TestCancelAfterManifestACKKeepsValidSnapshotAndStopsResume(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	if _, err := engine.Resume(context.Background(), scope, readyACK("resume", ready)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Cancel(context.Background(), scope, engineAction("keep-stopped", ready)); err != nil {
		t.Fatal(err)
	}
	if backend.deleted || backend.deletes != 0 || !backend.stopped {
		t.Fatal("canceling resume discarded an already ACKed snapshot")
	}
	final, err := engine.Finalize(context.Background(), scope, readyACK("finalize-stopped", ready))
	if err != nil || !final.Finalized || !final.ReadyToUse {
		t.Fatal("keep-stopped completion did not preserve ready snapshot")
	}
}

func TestSnapshotAuditDoesNotMarkNativeReadyOrHTTPAcceptanceAsSuccess(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	var events []dbmodel.ServiceEvent
	if err := engine.store.db.Where("event_id = ?", ready.EventID).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Status != "checking" || events[0].FinalStatus != "" {
		t.Fatal("HTTP acceptance/native Ready produced premature snapshot success")
	}
	if _, err := engine.Prepare(context.Background(), scope, SnapshotCreateRequest{Name: backend.fixture.intent.Name, Description: backend.fixture.intent.Description, RequestID: backend.fixture.op, ManifestID: "manifest-a", PowerAfter: "previous"}); err != nil {
		t.Fatal(err)
	}
	events = nil
	engine.store.db.Where("event_id = ?", ready.EventID).Find(&events)
	if len(events) != 1 {
		t.Fatal("retry created duplicate pending audit events")
	}
	if _, err := engine.Resume(context.Background(), scope, readyACK("resume", ready)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Finalize(context.Background(), scope, readyACK("finalize", ready)); err != nil {
		t.Fatal(err)
	}
	events = nil
	engine.store.db.Where("event_id = ?", ready.EventID).Find(&events)
	if len(events) != 1 || events[0].Status != "success" || events[0].FinalStatus != "complete" {
		t.Fatal("final ACK did not complete existing operation event")
	}
}

func setSnapshotFixtureOperation(t *testing.T, backend *fakeSnapshotBackend, operationID string) {
	t.Helper()
	f := snapshotProjectionFixture(t)
	f.op, f.intent.OperationID = operationID, operationID
	var err error
	f.core, err = SnapshotCoreHash(f.scope, f.intent)
	if err != nil {
		t.Fatal(err)
	}
	f.snapshot.Labels[SnapshotOperationLabel] = operationID
	f.snapshot.Annotations[SnapshotCoreAnnotation] = f.core
	backend.fixture = f
	setFakeSnapshotContents(backend)
	backend.creates, backend.deleted, backend.running, backend.stopped = 0, false, true, false
}

func setFakeSnapshotContents(backend *fakeSnapshotBackend) {
	backend.contents = map[string]*csi.VolumeSnapshotContent{}
	yes := true
	for _, volume := range backend.fixture.volumes {
		name := "csi-content-" + volume.Name
		volume.Status.BoundVolumeSnapshotContentName = &name
		backend.contents[name] = &csi.VolumeSnapshotContent{ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(name + "-uid")}, Spec: csi.VolumeSnapshotContentSpec{VolumeSnapshotRef: corev1.ObjectReference{APIVersion: csi.SchemeGroupVersion.String(), Kind: "VolumeSnapshot", Namespace: volume.Namespace, Name: volume.Name, UID: volume.UID}}, Status: &csi.VolumeSnapshotContentStatus{ReadyToUse: &yes}}
	}
}

func TestSnapshotCatalogNameReuseUsesCreationOrder(t *testing.T) {
	for _, tombstone := range []string{"deleted", "cancelled"} {
		t.Run(tombstone, func(t *testing.T) {
			engine, backend, scope := snapshotEngineFixture(t)
			setSnapshotFixtureOperation(t, backend, "z-old-operation")
			if tombstone == "deleted" {
				ready := engineReady(t, engine, scope, backend)
				if _, err := engine.Resume(context.Background(), scope, readyACK("resume-old", ready)); err != nil {
					t.Fatal(err)
				}
				if _, err := engine.Finalize(context.Background(), scope, readyACK("finalize-old", ready)); err != nil {
					t.Fatal(err)
				}
				request := engineAction("delete-old", ready)
				request.SnapshotUID = ready.SnapshotUID
				if _, err := engine.Delete(context.Background(), scope, request); err != nil {
					t.Fatal(err)
				}
				if deleted, err := engine.Delete(context.Background(), scope, request); err != nil || !deleted.Deleted {
					t.Fatalf("old snapshot did not reach deleted: %v", err)
				}
			} else {
				prepared := enginePrepare(t, engine, scope, backend)
				if _, err := engine.Cancel(context.Background(), scope, engineAction("cancel-old", prepared)); err != nil {
					t.Fatal(err)
				}
				ack := engineAction("resume-cancelled-old", prepared)
				ack.AckKind, ack.DocumentHash = "cancelled", strings.Repeat("e", 64)
				if _, err := engine.Resume(context.Background(), scope, ack); err != nil {
					t.Fatal(err)
				}
				ack.RequestID = "finalize-cancelled-old"
				if _, err := engine.Finalize(context.Background(), scope, ack); err != nil {
					t.Fatal(err)
				}
			}
			setSnapshotFixtureOperation(t, backend, "a-new-operation")
			newOperation := enginePrepare(t, engine, scope, backend)
			records, err := engine.records(scope)
			if err != nil || len(records) != 2 || records[0].OperationID != newOperation.OperationID {
				t.Fatalf("catalog lost creation order when operation names sort oppositely: %v", err)
			}
			list, err := engine.List(context.Background(), scope, 1, 20)
			if err != nil || list.Total != 1 || len(list.Items) != 1 || list.Items[0].OperationID != newOperation.OperationID {
				t.Fatalf("same-name tombstone hid the new live operation: %v", err)
			}
			byName, err := engine.ByName(context.Background(), scope, backend.fixture.intent.Name)
			if err != nil || byName.OperationID != newOperation.OperationID {
				t.Fatalf("name lookup selected the older tombstone: %v", err)
			}
		})
	}
}

func TestSnapshotCatalogRejectsMalformedDuplicateAndFutureReceipts(t *testing.T) {
	for _, corruption := range []string{"missing-applied", "future-step", "duplicate"} {
		t.Run(corruption, func(t *testing.T) {
			engine, backend, scope := snapshotEngineFixture(t)
			ready := engineReady(t, engine, scope, backend)
			if _, err := engine.Resume(context.Background(), scope, readyACK("resume", ready)); err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Finalize(context.Background(), scope, readyACK("finalize", ready)); err != nil {
				t.Fatal(err)
			}
			key, _ := ReceiptKey(ready.OperationID)
			var row dbmodel.ComponentK8sAttributes
			if err := engine.store.db.Where("component_id = ? AND name = ?", scope.ServiceID, key).First(&row).Error; err != nil {
				t.Fatal(err)
			}
			if corruption == "duplicate" {
				row.ID = 0
				if err := engine.store.db.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
			} else {
				var fields map[string]interface{}
				if err := json.Unmarshal([]byte(row.AttributeValue), &fields); err != nil {
					t.Fatal(err)
				}
				step := fields["steps"].(map[string]interface{})["snapshot-prepare"].(map[string]interface{})
				if corruption == "missing-applied" {
					delete(step, "applied")
				} else {
					step["control_revision"] = 999999
				}
				value, err := json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				if err := engine.store.db.Model(&row).Update("attribute_value", string(value)).Error; err != nil {
					t.Fatal(err)
				}
			}
			// An empty page must still validate all receipts before reporting its total.
			if _, err := engine.List(context.Background(), scope, 2, 20); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("catalog bypassed central persisted-receipt validation: %v", err)
			}
		})
	}
}

func TestSnapshotACKRejectsReplacedCSIIdentityBeforeFirstACK(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	request := readyACK("resume-persisted-manifest", ready)
	if len(ready.Disks) != 1 {
		t.Fatal("fixture lacks persisted CSI identity")
	}
	persistedUID := ready.Disks[0].VolumeSnapshot.UID
	for _, volume := range backend.fixture.volumes {
		volume.UID = types.UID("replacement-csi-before-ack")
	}
	result, err := engine.Resume(context.Background(), scope, request)
	if err == nil || backend.resumes != 0 {
		current, readErr := engine.store.Read(scope, ready.OperationID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		t.Fatalf("ACK accepted replacement CSI: persisted=%s sealed=%v resumed=%d acked=%v result_running=%v error=%v", persistedUID, proofFor(current.Receipt).VolumeSnapshotUIDs, backend.resumes, current.Receipt.ConsoleAckHash != "", result.SourceRunning, err)
	}
}

func TestSnapshotKeepStoppedPreventsLaterResume(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	if _, err := engine.Finalize(context.Background(), scope, readyACK("ack-before-resume", ready)); !errors.Is(err, ErrSnapshotPending) {
		t.Fatalf("ready ACK setup failed: %v", err)
	}
	if _, err := engine.Cancel(context.Background(), scope, engineAction("keep-stopped", ready)); err != nil {
		t.Fatal(err)
	}
	result, err := engine.Resume(context.Background(), scope, readyACK("resume-after-cancel", ready))
	if backend.resumes != 0 || result.SourceRunning {
		t.Fatalf("keep-stopped cancellation was overridden by later Resume: resumes=%d running=%v error=%v", backend.resumes, result.SourceRunning, err)
	}
}

func TestSnapshotACKPinsExactCSIContentAndLogicalPairing(t *testing.T) {
	for _, change := range []string{"missing-proof", "missing-content", "replaced-content", "replaced-volume-consistent", "wrong-pair"} {
		t.Run(change, func(t *testing.T) {
			engine, backend, scope := snapshotEngineFixture(t)
			ready := engineReady(t, engine, scope, backend)
			if !hashPattern.MatchString(ready.NativeProofHash) {
				t.Fatal("ready DTO lacks server-derived proof")
			}
			request := readyACK("resume-persisted-proof", ready)
			for name, content := range backend.contents {
				switch change {
				case "missing-proof":
					request.NativeProofHash = ""
				case "missing-content":
					delete(backend.contents, name)
				case "replaced-content":
					content.UID = "replacement-content"
				case "replaced-volume-consistent":
					backend.fixture.volumes[content.Spec.VolumeSnapshotRef.Name].UID = "replacement-volume"
					content.Spec.VolumeSnapshotRef.UID = "replacement-volume"
				case "wrong-pair":
					content.Spec.VolumeSnapshotRef.Name = "other-volume"
				}
			}
			if _, err := engine.Resume(context.Background(), scope, request); err == nil || backend.resumes != 0 {
				t.Fatalf("unpersisted or mismatched CSI proof allowed Start: %v", err)
			}
			current, err := engine.store.Read(scope, ready.OperationID)
			if err != nil || current.Receipt.ConsoleAckHash != "" || current.State.Fence == nil {
				t.Fatal("rejected proof changed ACK/fence")
			}
		})
	}
}

func TestSnapshotNativeProofReadsAreStableAndReadOnly(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	before, err := engine.store.Read(scope, ready.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		current, err := engine.Get(context.Background(), scope, ready.OperationID)
		if err != nil || current.NativeProofHash != ready.NativeProofHash {
			t.Fatal("proof depends on map traversal order")
		}
	}
	after, err := engine.store.Read(scope, ready.OperationID)
	if err != nil || before.State.ControlRevision != after.State.ControlRevision || after.Receipt.ConsoleAckHash != "" {
		t.Fatal("reading proof mutated receipt")
	}
}

func TestSnapshotKeepStoppedWaitsForInflightResumeThenObservedHalt(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	backend.resumeEntered, backend.resumeRelease = make(chan struct{}, 1), make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := engine.Resume(context.Background(), scope, readyACK("resume", ready))
		finished <- err
	}()
	select {
	case <-backend.resumeEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not enter barrier")
	}
	cancel := engineAction("keep-stopped", ready)
	pending, err := engine.Cancel(context.Background(), scope, cancel)
	if !errors.Is(err, ErrSnapshotPending) || pending.Cancelled || pending.Finalized || pending.ErrorCode != "vm_snapshot_resume_outcome_pending" {
		t.Fatalf("inflight Start was called cancelled: %+v %v", pending, err)
	}
	if _, err := engine.Finalize(context.Background(), scope, readyACK("finalize", ready)); !errors.Is(err, ErrSnapshotPending) {
		t.Fatal("inflight Start released fence")
	}
	close(backend.resumeRelease)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	current, err := engine.store.Read(scope, ready.OperationID)
	if err != nil || current.State.Fence == nil {
		t.Fatal("late Start completion released fence")
	}
	if _, kept := current.Receipt.Steps["snapshot-keep-stopped"]; !kept {
		t.Fatal("late Start erased keep-stopped")
	}
	if _, err := engine.Finalize(context.Background(), scope, readyACK("finalize", ready)); !errors.Is(err, ErrSnapshotPending) {
		t.Fatal("running source was declared stopped")
	}
	stopped, err := engine.Cancel(context.Background(), scope, cancel)
	if err != nil || !stopped.SourceStopped || stopped.SourceRunning || backend.deleted {
		t.Fatalf("settled Start could not be halted safely: %v", err)
	}
	final, err := engine.Finalize(context.Background(), scope, readyACK("finalize", ready))
	if err != nil || !final.Finalized || !final.ReadyToUse {
		t.Fatalf("observed halted valid snapshot did not finalize: %v", err)
	}
}

func TestSnapshotConcurrentResumeClaimsOnlyOneStart(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	backend.resumeEntered, backend.resumeRelease = make(chan struct{}, 2), make(chan struct{})
	finished := make(chan error, 1)
	request := readyACK("same-resume", ready)
	go func() { _, err := engine.Resume(context.Background(), scope, request); finished <- err }()
	select {
	case <-backend.resumeEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not enter barrier")
	}
	if _, err := engine.Resume(context.Background(), scope, request); !errors.Is(err, ErrSnapshotPending) {
		t.Fatalf("duplicate unsettled intent was not pending: %v", err)
	}
	close(backend.resumeRelease)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if backend.resumes != 1 {
		t.Fatal("concurrent retry repeated Start")
	}
}

func TestSnapshotUnknownResumeKeepsFenceAndCancellationIntent(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	backend.resumeError = ErrReferenceUnavailable
	request := readyACK("resume", ready)
	pending, err := engine.Resume(context.Background(), scope, request)
	if !errors.Is(err, ErrReferenceUnavailable) || pending.ErrorCode != "vm_snapshot_resume_outcome_pending" {
		t.Fatalf("unknown Start outcome was hidden: %v", err)
	}
	if _, err := engine.Resume(context.Background(), scope, request); !errors.Is(err, ErrSnapshotPending) || backend.resumes != 1 {
		t.Fatal("unknown Start was retried")
	}
	if _, err := engine.Cancel(context.Background(), scope, engineAction("keep-stopped", ready)); !errors.Is(err, ErrSnapshotPending) {
		t.Fatal("absent VMI settled unknown Start")
	}
	if _, err := engine.Resume(context.Background(), scope, request); !errors.Is(err, ErrConflict) {
		t.Fatal("keep-stopped was overridden")
	}
	if _, err := engine.Finalize(context.Background(), scope, readyACK("finalize", ready)); !errors.Is(err, ErrSnapshotPending) {
		t.Fatal("unknown Start released fence")
	}
	current, err := engine.store.Read(scope, ready.OperationID)
	if err != nil || current.State.Fence == nil || backend.deleted {
		t.Fatal("unknown Start lost fence or valid snapshot")
	}
}

func TestSnapshotFailedCommandsKeepFenceAndUnconfirmedSteps(t *testing.T) {
	for _, failure := range []string{"halt-unavailable", "halt-context", "halt-source-replaced", "create-context", "create-source-replaced", "observe-unavailable", "cleanup-unavailable", "resume-locked", "finalize-locked", "stop-after-ack-unavailable"} {
		t.Run(failure, func(t *testing.T) {
			engine, backend, scope := snapshotEngineFixture(t)
			prepared := enginePrepare(t, engine, scope, backend)
			var err error
			switch failure {
			case "halt-unavailable":
				backend.haltError = ErrReferenceUnavailable
				_, err = engine.Halt(context.Background(), scope, engineAction("halt", prepared))
			case "halt-context":
				backend.contextError = ErrReferenceUnavailable
				_, err = engine.Halt(context.Background(), scope, engineAction("halt", prepared))
			case "halt-source-replaced":
				backend.fixture.content.Spec.Source.VirtualMachine.UID = "replaced-vm"
				_, err = engine.Halt(context.Background(), scope, engineAction("halt", prepared))
			case "observe-unavailable":
				backend.observeError = ErrReferenceUnavailable
				_, err = engine.Get(context.Background(), scope, prepared.OperationID)
			default:
				if _, e := engine.Halt(context.Background(), scope, engineAction("halt", prepared)); e != nil {
					t.Fatal(e)
				}
				if failure == "create-context" {
					backend.contextError = ErrReferenceUnavailable
					_, err = engine.Create(context.Background(), scope, engineAction("create", prepared))
					break
				}
				if failure == "create-source-replaced" {
					backend.fixture.content.Spec.Source.VirtualMachine.Spec.Template.Spec.Domain.CPU.Cores = 2
					_, err = engine.Create(context.Background(), scope, engineAction("create", prepared))
					break
				}
				ready, e := engine.Create(context.Background(), scope, engineAction("create", prepared))
				if e != nil {
					t.Fatal(e)
				}
				switch failure {
				case "cleanup-unavailable":
					backend.deleteError = ErrReferenceUnavailable
					_, err = engine.Cancel(context.Background(), scope, engineAction("cancel", ready))
				case "resume-locked":
					backend.unlocked = false
					_, err = engine.Resume(context.Background(), scope, readyACK("resume", ready))
				case "finalize-locked":
					backend.unlocked = false
					_, err = engine.Finalize(context.Background(), scope, readyACK("finalize", ready))
				case "stop-after-ack-unavailable":
					if _, e := engine.Resume(context.Background(), scope, readyACK("resume", ready)); e != nil {
						t.Fatal(e)
					}
					backend.haltError = ErrReferenceUnavailable
					_, err = engine.Cancel(context.Background(), scope, engineAction("keep-stopped", ready))
				}
			}
			if err == nil {
				t.Fatal("failed command reported success")
			}
			current, e := engine.store.Read(scope, prepared.OperationID)
			if e != nil || current.State.Fence == nil {
				t.Fatal("failed command released durable fence")
			}
			if _, done := current.Receipt.Steps["snapshot-finalize"]; done {
				t.Fatal("failed command recorded final success")
			}
		})
	}
}

func TestSnapshotAuditRejectsDuplicateOrForeignOperationEvent(t *testing.T) {
	for _, corruption := range []string{"duplicate", "foreign"} {
		t.Run(corruption, func(t *testing.T) {
			engine, backend, scope := snapshotEngineFixture(t)
			prepared := enginePrepare(t, engine, scope, backend)
			var event dbmodel.ServiceEvent
			if err := engine.store.db.Where("event_id = ?", prepared.EventID).First(&event).Error; err != nil {
				t.Fatal(err)
			}
			if corruption == "duplicate" {
				event.ID = 0
				if err := engine.store.db.Create(&event).Error; err != nil {
					t.Fatal(err)
				}
			} else {
				if err := engine.store.db.Model(&event).Update("target_id", "other-service").Error; err != nil {
					t.Fatal(err)
				}
			}
			_, err := engine.Prepare(context.Background(), scope, SnapshotCreateRequest{Name: backend.fixture.intent.Name, Description: backend.fixture.intent.Description, RequestID: backend.fixture.op, ManifestID: "manifest-a", PowerAfter: "previous"})
			if err == nil {
				t.Fatal("corrupt audit identity was reused")
			}
		})
	}
}

func TestSnapshotDeleteStillBlocksUnreleasedReceiptReference(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	if _, err := engine.Resume(context.Background(), scope, readyACK("resume", ready)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Finalize(context.Background(), scope, readyACK("finalize", ready)); err != nil {
		t.Fatal(err)
	}
	current, err := engine.store.Read(scope, "")
	if err != nil {
		t.Fatal(err)
	}
	expected := Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, OperationID: "restore-reference", ManifestHash: strings.Repeat("a", 64), Step: "prepare", RequestHash: strings.Repeat("b", 64)}
	_, err = engine.store.Mutate(scope, expected, func(c Current) (Mutation, error) {
		r := NewReceipt(scope, expected.OperationID, expected.ManifestHash)
		r.Resources = []ResourceRef{{ObjectRef: refOf("VirtualMachineSnapshot", backend.fixture.snapshot), Purpose: "restore-source-snapshot"}}
		return Mutation{State: c.State, Receipt: &r}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request := engineAction("delete", ready)
	request.SnapshotUID = ready.SnapshotUID
	if _, err := engine.Delete(context.Background(), scope, request); !errors.Is(err, ErrBusy) || backend.deleted {
		t.Fatal("unreleased Region receipt reference was ignored")
	}
	current, err = engine.store.Read(scope, expected.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	expected.ControlRevision = current.State.ControlRevision
	expected.Step = "release"
	expected.RequestHash = strings.Repeat("c", 64)
	_, err = engine.store.Mutate(scope, expected, func(c Current) (Mutation, error) {
		c.Receipt.SnapshotReferenceReleased = true
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Delete(context.Background(), scope, request); err != nil {
		t.Fatal(err)
	}
	if deleted, err := engine.Delete(context.Background(), scope, request); err != nil || !deleted.Deleted {
		t.Fatal("released reference still blocked complete cleanup")
	}
}

type pausedSnapshotObservation struct {
	SnapshotBackend
	calls            atomic.Int32
	pauseAt          int32
	haltAcceptedOnly bool
	entered          chan struct{}
	release          chan struct{}
}

func (b *pausedSnapshotObservation) Observe(ctx context.Context, scope Scope, intent SnapshotIntent, known []ResourceRef) (SnapshotObservation, error) {
	pause := b.pauseAt
	if pause == 0 {
		pause = 1
	}
	if b.calls.Add(1) == pause {
		close(b.entered)
		<-b.release
	}
	return b.SnapshotBackend.Observe(ctx, scope, intent, known)
}

func TestSnapshotFinalizeRechecksInflightResumeAtRelease(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	ctx := context.Background()
	if _, err := engine.Finalize(ctx, scope, readyACK("setup-ack", ready)); !errors.Is(err, ErrSnapshotPending) {
		t.Fatalf("ACK setup: %v", err)
	}
	backend.resumeEntered, backend.resumeRelease = make(chan struct{}, 1), make(chan struct{})
	wrapper := &pausedSnapshotObservation{SnapshotBackend: backend, entered: make(chan struct{}), release: make(chan struct{})}
	engine.backend = wrapper
	type outcome struct {
		dto SnapshotOperationDTO
		err error
	}
	finalDone := make(chan outcome, 1)
	go func() {
		d, e := engine.Finalize(ctx, scope, readyACK("overlapping-finalize", ready))
		finalDone <- outcome{d, e}
	}()
	select {
	case <-wrapper.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Finalize failed to pause after its initial no-resume receipt read")
	}
	resumeDone := make(chan error, 1)
	go func() { _, err := engine.Resume(ctx, scope, readyACK("overlapping-resume", ready)); resumeDone <- err }()
	select {
	case <-backend.resumeEntered:
	case <-time.After(3 * time.Second):
		close(wrapper.release)
		t.Fatal("Start failed to enter barrier")
	}
	pending, err := engine.Cancel(ctx, scope, engineAction("overlapping-keep-stopped", ready))
	if !errors.Is(err, ErrSnapshotPending) || pending.Finalized {
		close(wrapper.release)
		close(backend.resumeRelease)
		t.Fatalf("cancel setup: %+v %v", pending, err)
	}
	close(wrapper.release)
	final := <-finalDone
	current, readErr := engine.store.Read(scope, ready.OperationID)
	if readErr != nil {
		close(backend.resumeRelease)
		<-resumeDone
		t.Fatal(readErr)
	}
	held := current.State.Fence != nil
	unsettled := resumeUnsettled(current.Receipt)
	close(backend.resumeRelease)
	startErr := <-resumeDone
	if final.dto.Finalized || !held || !errors.Is(final.err, ErrSnapshotPending) {
		t.Fatalf("stale Finalize released in-flight Start: finalized=%v fence_held=%v unsettled=%v finalize_error=%v late_start_error=%v", final.dto.Finalized, held, unsettled, final.err, startErr)
	}
}

func (b *pausedSnapshotObservation) Halt(ctx context.Context, scope Scope, intent SnapshotIntent) error {
	if b.haltAcceptedOnly {
		return nil
	}
	return b.SnapshotBackend.Halt(ctx, scope, intent)
}

func TestSnapshotFinalizeRechecksLaterKeepStoppedDecision(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	ctx := context.Background()
	if _, err := engine.Resume(ctx, scope, readyACK("settled-resume", ready)); err != nil {
		t.Fatal(err)
	}
	wrapper := &pausedSnapshotObservation{SnapshotBackend: backend, entered: make(chan struct{}), release: make(chan struct{}), pauseAt: 2, haltAcceptedOnly: true}
	engine.backend = wrapper
	type outcome struct {
		dto SnapshotOperationDTO
		err error
	}
	finalDone := make(chan outcome, 1)
	go func() {
		d, e := engine.Finalize(ctx, scope, readyACK("finalize-before-cancel", ready))
		finalDone <- outcome{d, e}
	}()
	select {
	case <-wrapper.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Finalize failed to pause after reading running power intent")
	}
	stopping, err := engine.Cancel(ctx, scope, engineAction("later-keep-stopped", ready))
	if err != nil || !stopping.SourceRunning {
		close(wrapper.release)
		t.Fatalf("accepted Stop setup: %+v %v", stopping, err)
	}
	close(wrapper.release)
	final := <-finalDone
	current, err := engine.store.Read(scope, ready.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	_, keptStopped := current.Receipt.Steps["snapshot-keep-stopped"]
	if final.dto.Finalized || current.State.Fence == nil || !errors.Is(final.err, ErrSnapshotPending) {
		t.Fatalf("stale Finalize ignored durable keep-stopped: finalized=%v fence_held=%v keep_stopped=%v source_running=%v finalize_error=%v", final.dto.Finalized, current.State.Fence != nil, keptStopped, final.dto.SourceRunning, final.err)
	}
}

func TestSnapshotFinalizeWithUnchangedControlRevisionCompletes(t *testing.T) {
	for _, desiredPower := range []string{"running", "keep-stopped"} {
		t.Run(desiredPower, func(t *testing.T) {
			engine, backend, scope := snapshotEngineFixture(t)
			ready := engineReady(t, engine, scope, backend)
			ctx := context.Background()
			if desiredPower == "running" {
				if _, err := engine.Resume(ctx, scope, readyACK("resume", ready)); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := engine.Finalize(ctx, scope, readyACK("setup-ack", ready)); !errors.Is(err, ErrSnapshotPending) {
					t.Fatal(err)
				}
				if _, err := engine.Cancel(ctx, scope, engineAction("keep-stopped", ready)); err != nil {
					t.Fatal(err)
				}
			}
			before, err := engine.store.Read(scope, ready.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			finalized, err := engine.Finalize(ctx, scope, readyACK("finalize", ready))
			if err != nil || !finalized.Finalized || finalized.NativeProofHash != ready.NativeProofHash {
				t.Fatalf("unchanged observation/control state could not finalize: %v", err)
			}
			after, err := engine.store.Read(scope, ready.OperationID)
			if err != nil || after.State.Fence != nil || after.State.ControlRevision != before.State.ControlRevision+1 || after.Receipt.Steps["snapshot-finalize"].ControlRevision != after.State.ControlRevision {
				t.Fatal("terminal CAS did not advance exactly the observed control revision")
			}
			if desiredPower == "keep-stopped" && (!finalized.SourceStopped || finalized.SourceRunning) {
				t.Fatal("keep-stopped finalization lost the observed halted state")
			}
		})
	}
}

func TestSnapshotFinalizeRechecksFinalObservationNativeProof(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	ctx := context.Background()
	if _, err := engine.Resume(ctx, scope, readyACK("resume", ready)); err != nil {
		t.Fatal(err)
	}
	wrapper := &pausedSnapshotObservation{SnapshotBackend: backend, entered: make(chan struct{}), release: make(chan struct{}), pauseAt: 2}
	engine.backend = wrapper
	finished := make(chan error, 1)
	go func() {
		_, err := engine.Finalize(ctx, scope, readyACK("finalize", ready))
		finished <- err
	}()
	select {
	case <-wrapper.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Finalize did not reach its terminal observation")
	}
	backend.mu.Lock()
	for _, content := range backend.contents {
		content.UID = "replacement-after-ack-observation"
	}
	backend.mu.Unlock()
	close(wrapper.release)
	if err := <-finished; !errors.Is(err, ErrConflict) {
		t.Fatalf("terminal observation used a different native proof from the durable ACK: %v", err)
	}
	current, err := engine.store.Read(scope, ready.OperationID)
	if err != nil || current.State.Fence == nil {
		t.Fatal("changed terminal artifact proof released the fence")
	}
}

type snapshotCancelRaceBackend struct {
	*fakeSnapshotBackend
	guard          sync.Mutex
	pauseObserve   bool
	observeEntered chan struct{}
	observeRelease chan struct{}
	createEntered  chan struct{}
	createRelease  chan struct{}
}

func (b *snapshotCancelRaceBackend) Observe(ctx context.Context, scope Scope, intent SnapshotIntent, refs []ResourceRef) (SnapshotObservation, error) {
	observed, err := b.fakeSnapshotBackend.Observe(ctx, scope, intent, refs)
	b.guard.Lock()
	pause := b.pauseObserve
	b.pauseObserve = false
	b.guard.Unlock()
	if pause {
		close(b.observeEntered)
		<-b.observeRelease
	}
	return observed, err
}
func (b *snapshotCancelRaceBackend) Create(ctx context.Context, scope Scope, intent SnapshotIntent, operationID, coreHash string) (ObjectRef, error) {
	if b.createEntered != nil {
		close(b.createEntered)
		<-b.createRelease
	}
	return b.fakeSnapshotBackend.Create(ctx, scope, intent, operationID, coreHash)
}
func armSnapshotCancelRace(backend *fakeSnapshotBackend) *snapshotCancelRaceBackend {
	return &snapshotCancelRaceBackend{fakeSnapshotBackend: backend, pauseObserve: true, observeEntered: make(chan struct{}), observeRelease: make(chan struct{})}
}

func TestSnapshotCancelCannotReleaseFenceDuringLateNativeCreate(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	prepared := enginePrepare(t, engine, scope, backend)
	if _, err := engine.Halt(context.Background(), scope, engineAction("halt", prepared)); err != nil {
		t.Fatal(err)
	}
	gate := armSnapshotCancelRace(backend)
	gate.createEntered, gate.createRelease = make(chan struct{}), make(chan struct{})
	engine.backend = gate
	cancelDone := make(chan error, 1)
	go func() {
		_, err := engine.Cancel(context.Background(), scope, engineAction("cancel", prepared))
		cancelDone <- err
	}()
	<-gate.observeEntered // Cancel captured no native artifact and no Create intent.
	createDone := make(chan error, 1)
	go func() {
		_, err := engine.Create(context.Background(), scope, engineAction("create", prepared))
		createDone <- err
	}()
	<-gate.createEntered // Actual Store now has durable Create intent, call unsettled.
	close(gate.observeRelease)
	cancelErr := <-cancelDone
	if cancelErr != nil {
		close(gate.createRelease)
		<-createDone
		t.Logf("Cancel safely held: %v", cancelErr)
		return
	}
	// Trusted Console stores the cancelled document and sends its cancellation ACK.
	ack := engineAction("resume-cancelled", prepared)
	ack.AckKind = "cancelled"
	ack.DocumentHash = strings.Repeat("e", 64)
	_, resumeErr := engine.Resume(context.Background(), scope, ack)
	ack.RequestID = "finalize-cancelled"
	final, finalErr := engine.Finalize(context.Background(), scope, ack)
	current, readErr := engine.store.Read(scope, prepared.OperationID)
	close(gate.createRelease)
	createErr := <-createDone
	backend.mu.Lock()
	creates, running := backend.creates, backend.running
	backend.mu.Unlock()
	t.Logf("cancel=%v resume=%v finalize=%v finalized=%v fence_nil=%v read=%v late_create=%v actual_created=%d source_running=%v", cancelErr, resumeErr, finalErr, final.Finalized, current.State.Fence == nil, readErr, createErr, creates, running)
	if finalErr == nil && current.State.Fence == nil && creates > 0 {
		t.Fatal("cancelled/finalized operation released fence and resumed source before unsettled native Create materialized an untracked snapshot")
	}
}

func TestSnapshotStaleCancelCannotDeleteDurablyACKedSnapshot(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	gate := armSnapshotCancelRace(backend)
	engine.backend = gate
	cancelDone := make(chan error, 1)
	go func() {
		_, err := engine.Cancel(context.Background(), scope, engineAction("cancel", ready))
		cancelDone <- err
	}()
	<-gate.observeEntered // Cancel read receipt with no ACK, but is not yet mutating.
	resumed, resumeErr := engine.Resume(context.Background(), scope, readyACK("resume", ready))
	if resumeErr != nil {
		close(gate.observeRelease)
		<-cancelDone
		t.Fatal(resumeErr)
	}
	current, err := engine.store.Read(scope, ready.OperationID)
	if err != nil || current.Receipt.ConsoleAckHash == "" {
		close(gate.observeRelease)
		<-cancelDone
		t.Fatal("ACK was not persisted")
	}
	close(gate.observeRelease)
	cancelErr := <-cancelDone
	backend.mu.Lock()
	deleted, deletes := backend.deleted, backend.deletes
	backend.mu.Unlock()
	latest, readErr := engine.store.Read(scope, ready.OperationID)
	_, cancelIntent := latest.Receipt.Steps["snapshot-cancel-intent"]
	t.Logf("resume_running=%v ack=%v cancel=%v deleted=%v delete_calls=%d destructive_cancel_intent=%v read=%v", resumed.SourceRunning, latest.Receipt.ConsoleAckHash != "", cancelErr, deleted, deletes, cancelIntent, readErr)
	if deleted || deletes > 0 {
		t.Fatal("stale pre-ACK Cancel deleted the now durably ACKed complete snapshot instead of preserving it and keeping source stopped")
	}
}

type snapshotRPCGate struct {
	SnapshotBackend
	kind               string
	entered            chan struct{}
	release            chan struct{}
	calls              atomic.Int32
	deleteBeforeReturn bool
}

func (b *snapshotRPCGate) wait() {
	if b.calls.Add(1) == 1 {
		close(b.entered)
	}
	<-b.release
}
func (b *snapshotRPCGate) Create(ctx context.Context, scope Scope, intent SnapshotIntent, operationID, coreHash string) (ObjectRef, error) {
	if b.kind == "create" {
		b.wait()
	}
	return b.SnapshotBackend.Create(ctx, scope, intent, operationID, coreHash)
}
func (b *snapshotRPCGate) Halt(ctx context.Context, scope Scope, intent SnapshotIntent) error {
	if b.kind == "halt" {
		b.wait()
	}
	return b.SnapshotBackend.Halt(ctx, scope, intent)
}
func (b *snapshotRPCGate) Delete(ctx context.Context, scope Scope, refs []ResourceRef) error {
	if b.kind != "delete" {
		return b.SnapshotBackend.Delete(ctx, scope, refs)
	}
	if b.deleteBeforeReturn {
		err := b.SnapshotBackend.Delete(ctx, scope, refs)
		b.wait()
		return err
	}
	b.wait()
	return b.SnapshotBackend.Delete(ctx, scope, refs)
}

func TestSnapshotManagedDeleteCannotCompleteWhileRPCUnsettled(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	ctx := context.Background()
	if _, err := engine.Resume(ctx, scope, readyACK("resume", ready)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Finalize(ctx, scope, readyACK("finalize", ready)); err != nil {
		t.Fatal(err)
	}
	gate := &snapshotRPCGate{SnapshotBackend: backend, kind: "delete", entered: make(chan struct{}), release: make(chan struct{}), deleteBeforeReturn: true}
	engine.backend = gate
	request := engineAction("delete", ready)
	request.SnapshotUID = ready.SnapshotUID
	done := make(chan error, 1)
	go func() { _, err := engine.Delete(ctx, scope, request); done <- err }()
	<-gate.entered
	result, err := engine.Delete(ctx, scope, request)
	current, readErr := engine.store.Read(scope, ready.OperationID)
	close(gate.release)
	lateErr := <-done
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err == nil || result.Deleted || current.State.Fence == nil {
		t.Fatalf("missing artifacts released an unsettled delete RPC: deleted=%v fence_held=%v err=%v late=%v", result.Deleted, current.State.Fence != nil, err, lateErr)
	}
	deleted, err := engine.Delete(ctx, scope, request)
	if err != nil || !deleted.Deleted {
		t.Fatalf("settled deletion could not finish on fresh evidence: %v", err)
	}
}

func TestSnapshotConcurrentCreateClaimsOnlyOneRPC(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	prepared := enginePrepare(t, engine, scope, backend)
	ctx := context.Background()
	if _, err := engine.Halt(ctx, scope, engineAction("halt", prepared)); err != nil {
		t.Fatal(err)
	}
	gate := &snapshotRPCGate{SnapshotBackend: backend, kind: "create", entered: make(chan struct{}), release: make(chan struct{})}
	engine.backend = gate
	request := engineAction("create", prepared)
	first := make(chan error, 1)
	go func() { _, err := engine.Create(ctx, scope, request); first <- err }()
	<-gate.entered
	second := make(chan error, 1)
	go func() { _, err := engine.Create(ctx, scope, request); second <- err }()
	select {
	case err := <-second:
		if !errors.Is(err, ErrSnapshotPending) {
			close(gate.release)
			<-first
			t.Fatalf("duplicate claim was not pending: %v", err)
		}
	case <-time.After(3 * time.Second):
		close(gate.release)
		<-first
		<-second
		t.Fatal("second Create repeated backend RPC")
	}
	close(gate.release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if gate.calls.Load() != 1 || backend.creates != 1 {
		t.Fatal("concurrent Create repeated its external effect")
	}
}

func TestSnapshotLateCreateSettlesBeforeCleanupAndCancelledACK(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	prepared := enginePrepare(t, engine, scope, backend)
	ctx := context.Background()
	if _, err := engine.Halt(ctx, scope, engineAction("halt", prepared)); err != nil {
		t.Fatal(err)
	}
	gate := &snapshotRPCGate{SnapshotBackend: backend, kind: "create", entered: make(chan struct{}), release: make(chan struct{})}
	engine.backend = gate
	created := make(chan error, 1)
	go func() { _, err := engine.Create(ctx, scope, engineAction("create", prepared)); created <- err }()
	<-gate.entered
	request := engineAction("cancel", prepared)
	pending, err := engine.Cancel(ctx, scope, request)
	if err == nil || pending.Cancelled || pending.ErrorCode != "vm_snapshot_create_outcome_pending" {
		close(gate.release)
		<-created
		t.Fatal("unsettled Create was called cancelled")
	}
	ack := engineAction("resume-cancelled", prepared)
	ack.AckKind = "cancelled"
	ack.DocumentHash = strings.Repeat("e", 64)
	if _, err := engine.Resume(ctx, scope, ack); err == nil {
		close(gate.release)
		<-created
		t.Fatal("cancelled ACK started VM before Create settled")
	}
	close(gate.release)
	if err := <-created; err != nil {
		t.Fatal(err)
	}
	current, err := engine.store.Read(scope, prepared.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if current.State.Fence == nil || !snapshotStep(current.Receipt, "snapshot-created") || proofFor(current.Receipt).SnapshotUID != string(backend.fixture.snapshot.UID) || current.State.Fence.Phase != "cancelling" {
		t.Fatal("late Create lost its exact UID or cancellation phase")
	}
	if _, err := engine.Cancel(ctx, scope, request); err != nil {
		t.Fatal(err)
	}
	clean, err := engine.Cancel(ctx, scope, request)
	if err != nil || !clean.Cancelled || !backend.deleted {
		t.Fatalf("settled artifact was not actually cleaned: %v", err)
	}
	if _, err := engine.Resume(ctx, scope, ack); err != nil {
		t.Fatal(err)
	}
	ack.RequestID = "finalize-cancelled"
	if final, err := engine.Finalize(ctx, scope, ack); err != nil || !final.Finalized {
		t.Fatalf("cleaned and ACKed cancellation did not finalize: %v", err)
	}
}

func TestSnapshotCleanupChoiceRejectsLaterReadyACKAndCreate(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	ctx := context.Background()
	gate := &pausedSnapshotObservation{SnapshotBackend: backend, pauseAt: 2, entered: make(chan struct{}), release: make(chan struct{})}
	engine.backend = gate
	done := make(chan error, 1)
	go func() { _, err := engine.Cancel(ctx, scope, engineAction("cancel", ready)); done <- err }()
	<-gate.entered
	if _, err := engine.Resume(ctx, scope, readyACK("late-ready-ack", ready)); err == nil {
		close(gate.release)
		<-done
		t.Fatal("ready ACK crossed the durable cleanup choice")
	}
	current, err := engine.store.Read(scope, ready.OperationID)
	if err != nil {
		close(gate.release)
		<-done
		t.Fatal(err)
	}
	if current.Receipt.ConsoleAckHash != "" || backend.resumes != 0 {
		close(gate.release)
		<-done
		t.Fatal("cleanup choice was replaced by ready ACK")
	}
	close(gate.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// A separately prepared operation cancelled before Create must never issue Create.
	other, otherBackend, otherScope := snapshotEngineFixture(t)
	prepared := enginePrepare(t, other, otherScope, otherBackend)
	if _, err := other.Cancel(ctx, otherScope, engineAction("cancel-before-create", prepared)); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Create(ctx, otherScope, engineAction("create", prepared)); err == nil || otherBackend.creates != 0 {
		t.Fatal("Create started after cancellation chose cleanup")
	}
}

func TestSnapshotUnknownHaltAndDeleteNeverRetryOrCompleteOnAbsence(t *testing.T) {
	for _, effect := range []string{"halt", "cleanup-delete", "managed-delete"} {
		t.Run(effect, func(t *testing.T) {
			engine, backend, scope := snapshotEngineFixture(t)
			ctx := context.Background()
			if effect == "halt" {
				prepared := enginePrepare(t, engine, scope, backend)
				backend.haltError = ErrReferenceUnavailable
				if _, err := engine.Halt(ctx, scope, engineAction("halt", prepared)); !errors.Is(err, ErrReferenceUnavailable) {
					t.Fatal(err)
				}
				if _, err := engine.Halt(ctx, scope, engineAction("halt", prepared)); !errors.Is(err, ErrSnapshotPending) || backend.halts != 1 {
					t.Fatal("unknown Halt was retried")
				}
				backend.stopped, backend.running = true, false
				pending, err := engine.Cancel(ctx, scope, engineAction("cancel", prepared))
				if err == nil || pending.Cancelled || pending.ErrorCode != "vm_snapshot_halt_outcome_pending" {
					t.Fatal("absent VMI settled unknown Halt")
				}
				return
			}
			ready := engineReady(t, engine, scope, backend)
			if effect == "managed-delete" {
				if _, err := engine.Resume(ctx, scope, readyACK("resume", ready)); err != nil {
					t.Fatal(err)
				}
				if _, err := engine.Finalize(ctx, scope, readyACK("finalize", ready)); err != nil {
					t.Fatal(err)
				}
			}
			backend.deleteError = ErrReferenceUnavailable
			request := engineAction("delete", ready)
			request.SnapshotUID = ready.SnapshotUID
			invoke := func() (SnapshotOperationDTO, error) {
				if effect == "managed-delete" {
					return engine.Delete(ctx, scope, request)
				}
				return engine.Cancel(ctx, scope, request)
			}
			if _, err := invoke(); !errors.Is(err, ErrReferenceUnavailable) {
				t.Fatal(err)
			}
			backend.deleted = true
			pending, err := invoke()
			if err == nil || pending.Deleted || pending.Cancelled || backend.deletes != 1 {
				t.Fatal("absence completed or retried an unknown delete RPC")
			}
			want := "vm_snapshot_cleanup_outcome_pending"
			if effect == "managed-delete" {
				want = "vm_snapshot_delete_outcome_pending"
			}
			if pending.ErrorCode != want {
				t.Fatalf("unknown effect reason=%s", pending.ErrorCode)
			}
			current, err := engine.store.Read(scope, ready.OperationID)
			if err != nil || current.State.Fence == nil {
				t.Fatal("unknown delete lost its fence")
			}
		})
	}
}

func TestSnapshotAcceptedHaltMustBeObservedBeforeCancellationResume(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	prepared := enginePrepare(t, engine, scope, backend)
	ctx := context.Background()
	backend.haltAcceptedOnly = true
	if _, err := engine.Halt(ctx, scope, engineAction("halt", prepared)); err != nil {
		t.Fatal(err)
	}
	request := engineAction("cancel", prepared)
	if pending, err := engine.Cancel(ctx, scope, request); !errors.Is(err, ErrSnapshotPending) || pending.Cancelled {
		t.Fatal("accepted Stop was mistaken for observed halted cleanup")
	}
	backend.running, backend.stopped = false, true
	if clean, err := engine.Cancel(ctx, scope, request); err != nil || !clean.Cancelled {
		t.Fatalf("observed Halt could not complete cleanup: %v", err)
	}
}

func TestSnapshotStaleReadyACKCannotCrossCleanupSelection(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	ctx := context.Background()
	gate := armSnapshotCancelRace(backend)
	engine.backend = gate
	done := make(chan error, 1)
	go func() { _, err := engine.Resume(ctx, scope, readyACK("stale-ready-ack", ready)); done <- err }()
	<-gate.observeEntered
	if _, err := engine.Cancel(ctx, scope, engineAction("cancel", ready)); err != nil {
		close(gate.observeRelease)
		<-done
		t.Fatal(err)
	}
	close(gate.observeRelease)
	if err := <-done; err == nil {
		t.Fatal("pre-choice observation committed a later ready ACK")
	}
	current, err := engine.store.Read(scope, ready.OperationID)
	if err != nil || current.Receipt.ConsoleAckHash != "" || backend.resumes != 0 {
		t.Fatal("cleanup choice lost its atomic ACK exclusion")
	}
}

func TestSnapshotIdempotencyAndPendingHaltProtectSideEffects(t *testing.T) {
	ctx := context.Background()
	t.Run("halt-replay-and-cancel", func(t *testing.T) {
		engine, backend, scope := snapshotEngineFixture(t)
		backend.running, backend.stopped = false, true
		prepared := enginePrepare(t, engine, scope, backend)
		request := engineAction("halt", prepared)
		if _, err := engine.Halt(ctx, scope, request); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Halt(ctx, scope, request); err != nil || backend.halts != 0 {
			t.Fatal("already halted replay issued Stop")
		}
		cancel := engineAction("cancel", prepared)
		if _, err := engine.Cancel(ctx, scope, cancel); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Halt(ctx, scope, request); !errors.Is(err, ErrConflict) {
			t.Fatal("Halt crossed cancellation selection")
		}
		cancel.RequestID = "different-cancel"
		if _, err := engine.Cancel(ctx, scope, cancel); !errors.Is(err, ErrConflict) {
			t.Fatal("terminal cancellation accepted a different request identity")
		}
	})
	t.Run("create-after-unknown-halt", func(t *testing.T) {
		engine, backend, scope := snapshotEngineFixture(t)
		prepared := enginePrepare(t, engine, scope, backend)
		backend.haltError = ErrReferenceUnavailable
		if _, err := engine.Halt(ctx, scope, engineAction("halt", prepared)); !errors.Is(err, ErrReferenceUnavailable) {
			t.Fatal(err)
		}
		backend.running, backend.stopped = false, true
		if _, err := engine.Create(ctx, scope, engineAction("create", prepared)); !errors.Is(err, ErrSnapshotPending) || backend.creates != 0 {
			t.Fatal("absent VMI allowed Create before unknown Stop settled")
		}
	})
	t.Run("create-identity-reuse", func(t *testing.T) {
		engine, backend, scope := snapshotEngineFixture(t)
		ready := engineReady(t, engine, scope, backend)
		if _, err := engine.Create(ctx, scope, engineAction("different-create", ready)); !errors.Is(err, ErrConflict) || backend.creates != 1 {
			t.Fatal("different replay identity repeated Create")
		}
	})
}

func TestSnapshotDeleteAndListRejectInvalidTerminalRequests(t *testing.T) {
	engine, backend, scope := snapshotEngineFixture(t)
	ready := engineReady(t, engine, scope, backend)
	ctx := context.Background()
	request := engineAction("delete", ready)
	request.SnapshotUID = ready.SnapshotUID
	if _, err := engine.Delete(ctx, scope, request); !errors.Is(err, ErrBusy) {
		t.Fatal("unfinalized snapshot accepted managed deletion")
	}
	if _, err := engine.Resume(ctx, scope, readyACK("resume", ready)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Finalize(ctx, scope, readyACK("finalize", ready)); err != nil {
		t.Fatal(err)
	}
	wrong := request
	wrong.SnapshotUID = "replacement-uid"
	if _, err := engine.Delete(ctx, scope, wrong); !errors.Is(err, ErrConflict) || backend.deletes != 0 {
		t.Fatal("wrong UID issued Delete")
	}
	if _, err := engine.Delete(ctx, scope, request); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Delete(ctx, scope, request); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "different-delete"
	if _, err := engine.Delete(ctx, scope, request); !errors.Is(err, ErrConflict) || backend.deletes != 1 {
		t.Fatal("terminal replay accepted a different identity")
	}
	if _, err := engine.List(ctx, scope, 0, 20); !errors.Is(err, ErrSnapshotInvalid) {
		t.Fatal("invalid pagination was accepted")
	}
	if _, err := engine.ByName(ctx, scope, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing managed name did not return not found")
	}
}
