package vmrestore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	corev1 "k8s.io/api/core/v1"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func resourceGenerationDrift(t *testing.T, f restoreCommitFixture) {
	t.Helper()
	_, err := f.store.Mutate(f.scope, f.expectStep(t, "unexpected-resource-generation"), func(c Current) (Mutation, error) {
		c.State.BindingGeneration++
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-restore.resource-execution
func TestResourceExecutorRefusesGenerationDriftBeforeExternalMutation(t *testing.T) {
	for _, when := range []string{"before-create", "inside-check", "before-delete", "inside-delete-check"} {
		t.Run(when, func(t *testing.T) {
			f, frame, executor := resourceFixture(t)
			cm := &corev1.ConfigMap{ObjectMeta: frame.metadata(frame.name(0, "inspect-plan"), "restore-plan"), Data: map[string]string{"plan.json": "fixture"}}
			if strings.Contains(when, "delete") {
				if _, err := executor.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", cm); err != nil {
					t.Fatal(err)
				}
			}
			if when == "inside-check" || when == "inside-delete-check" {
				executor.BeforeMutation = func(context.Context, Scope, string, string) error { resourceGenerationDrift(t, f); return nil }
			} else {
				resourceGenerationDrift(t, f)
			}
			executor.Dynamic.(*dynamicfake.FakeDynamicClient).ClearActions()
			var err error
			if strings.Contains(when, "delete") {
				err = executor.Delete(context.Background(), f.scope, f.expect.OperationID, "inspect-plan")
			} else {
				_, err = executor.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", cm)
			}
			if !errors.Is(err, ErrConflict) {
				t.Fatalf("generation drift was not rejected: %v", err)
			}
			for _, action := range executor.Dynamic.(*dynamicfake.FakeDynamicClient).Actions() {
				if action.GetVerb() == "create" || action.GetVerb() == "delete" {
					t.Fatal("stale generation reached resource mutation")
				}
			}
		})
	}
}

func TestExecutionInitializationRejectsInvalidSafeStartupAndReceiptIdentity(t *testing.T) {
	for _, kind := range []string{"uuid", "mode", "datasource", "producer", "paired-provenance", "media-namespace", "media-kind", "media-duplicate", "evidence-mode", "evidence-scope", "intent-nonce", "intent-pv", "nil-maps", "previous-uid", "foreign-volume"} {
		t.Run(kind, func(t *testing.T) {
			model, _, _ := resourceFixture(t)
			c, err := model.store.Read(model.scope, model.expect.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(c.Receipt.Execution)
			var x RestoreExecution
			if err := json.Unmarshal(data, &x); err != nil {
				t.Fatal(err)
			}
			f := newRestoreCommitFixture(t)
			f.prepare(t)
			media := RestoreGeneratedMedia{VolumeName: "generated-env", DiskName: "generated-env", ProducerUID: "producer-uid", Source: ObjectRef{Kind: "ConfigMap", Namespace: f.scope.Namespace, Name: "generated-env", UID: "env-uid"}}
			switch kind {
			case "uuid":
				x.BeforeStartup.FirmwareUUID = "unknown"
			case "mode":
				x.BeforeStartup.BootstrapMode = "unknown"
			case "datasource":
				x.BeforeStartup.DataSourceKind = "other"
			case "producer":
				x.BeforeStartup.ProducerUID = ""
			case "paired-provenance":
				x.BeforeStartup.RestoreID = "previous-restore"
			case "media-namespace":
				media.Source.Namespace = "foreign"
				x.BeforeStartup.GeneratedMedia = []RestoreGeneratedMedia{media}
			case "media-kind":
				media.Source.Kind = "Pod"
				x.BeforeStartup.GeneratedMedia = []RestoreGeneratedMedia{media}
			case "media-duplicate":
				x.BeforeStartup.GeneratedMedia = []RestoreGeneratedMedia{media, media}
			case "evidence-mode":
				x.Evidence["other"] = restoreplan.CopyEvidence{}
			case "evidence-scope":
				x.Evidence["copy"] = restoreplan.CopyEvidence{Identity: restoreplan.ResultIdentity{Namespace: "foreign"}, LogSHA256: strings.Repeat("a", 64)}
			case "intent-nonce":
				x.Intents["test"] = RestoreObjectIntent{Key: "test", Object: ObjectRef{Kind: "ConfigMap", Namespace: f.scope.Namespace, Name: "test"}, DesiredHash: strings.Repeat("a", 64), Purpose: "restore-plan"}
			case "intent-pv":
				x.Intents["test"] = RestoreObjectIntent{Key: "test", Nonce: "nonce", Object: ObjectRef{Kind: "ConfigMap", Namespace: f.scope.Namespace, Name: "test"}, DesiredHash: strings.Repeat("a", 64), Purpose: "restore-plan", BoundPV: &ObjectRef{Kind: "PersistentVolume", Namespace: "foreign", Name: "pv", UID: "pv-uid"}}
			case "nil-maps":
				x.Intents = nil
			case "previous-uid":
				x.Frame.Disks[0].PreviousPVC.UID = "different-previous-uid"
				x.Frame.FrameHash, _ = RestoreResourceFrameHash(x.Frame)
			case "foreign-volume":
				x.Frame.Disks[0].VolumeID = 999
				x.Frame.FrameHash, _ = RestoreResourceFrameHash(x.Frame)
			}
			before, err := f.store.Read(f.scope, f.expect.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.store.InitializeRestoreExecution(f.scope, f.expect.OperationID, x); err == nil {
				t.Fatal("invalid startup/receipt source was accepted")
			}
			after, err := f.store.Read(f.scope, f.expect.OperationID)
			if err != nil || after.State.ControlRevision != before.State.ControlRevision || after.Receipt.Execution != nil {
				t.Fatal("rejected execution initialization changed SQL")
			}
		})
	}
}

func TestResourceIntentRetryAfterSafetyCheckAndMonotonicDelete(t *testing.T) {
	f, frame, e := resourceFixture(t)
	cm := &corev1.ConfigMap{ObjectMeta: frame.metadata(frame.name(0, "inspect-plan"), "restore-plan"), Data: map[string]string{"plan.json": "fixture"}}
	e.BeforeMutation = func(context.Context, Scope, string, string) error { return ErrRestorePending }
	if _, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", cm); !errors.Is(err, ErrRestorePending) {
		t.Fatal(err)
	}
	c, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil || c.Receipt.Execution.Intents["inspect-plan"].Nonce == "" {
		t.Fatal("failed safety check lost create intent")
	}
	nonce := c.Receipt.Execution.Intents["inspect-plan"].Nonce
	e.BeforeMutation = func(context.Context, Scope, string, string) error { return nil }
	ref, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", cm)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "inspect-plan", "restore-plan", "ConfigMap", cm)
	if err != nil || ref != replayed {
		t.Fatal("known UID did not replay")
	}
	c, err = f.store.Read(f.scope, f.expect.OperationID)
	if err != nil || c.Receipt.Execution.Intents["inspect-plan"].Nonce != nonce {
		t.Fatal("retry replaced nonce")
	}
	if err := e.Delete(context.Background(), f.scope, f.expect.OperationID, "inspect-plan"); !errors.Is(err, ErrRestorePending) {
		t.Fatal(err)
	}
	if err := e.Delete(context.Background(), f.scope, f.expect.OperationID, "inspect-plan"); err != nil {
		t.Fatal(err)
	}
	if err := e.Delete(context.Background(), f.scope, f.expect.OperationID, "inspect-plan"); err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Mutate(f.scope, f.expectStep(t, "erase-resource-history"), func(c Current) (Mutation, error) {
		v := c.Receipt.Execution.Intents["inspect-plan"]
		v.Deleted = false
		v.DeleteRequested = false
		c.Receipt.Execution.Intents["inspect-plan"] = v
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatal("cleanup history regressed")
	}
	if c, err := f.store.Read(f.scope, f.expect.OperationID); err != nil || c.State.Fence == nil || len(c.Receipt.OldBindings) != 3 {
		t.Fatal("temporary cleanup lost fence or pinned original B")
	}
}

func TestResourceCleanupUsesSealedCommitAndRollbackGeneration(t *testing.T) {
	f, _, executor := resourceFixture(t)
	f.sealCopy(t)
	if _, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, RestoreCommitStep), f.request(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := executor.current(context.Background(), f.scope, f.expect.OperationID); err != nil {
		t.Fatalf("sealed generation blocked post-commit cleanup: %v", err)
	}
	f.rollbackReady(t)
	if _, err := f.store.RollbackRestoreDB(f.scope, f.expectStep(t, "restore-rollback")); err != nil {
		t.Fatal(err)
	}
	if _, err := executor.current(context.Background(), f.scope, f.expect.OperationID); err != nil {
		t.Fatalf("sealed rollback generation blocked cleanup: %v", err)
	}
}

func TestCopyEvidenceSQLFailureAndReplacementCannotBecomeSuccess(t *testing.T) {
	f, frame, _ := resourceFixture(t)
	evidence := restoreplan.CopyEvidence{Identity: restoreplan.ResultIdentity{Namespace: f.scope.Namespace, JobUID: "job-uid", PodUID: "pod-uid", FrameHash: frame.FrameHash}, LogBytes: 12, LogSHA256: strings.Repeat("a", 64), Report: &copyio.Report{Protocol: copyio.Protocol, OperationID: f.expect.OperationID, Phase: "source-scan", Mode: "inspect", ErrorCode: "copy_io_error"}}
	before, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	trigger := `CREATE TRIGGER reject_copy_evidence BEFORE UPDATE ON component_k8s_attributes WHEN NEW.name LIKE 'vm_restore_receipt_%' AND NEW.attribute_value LIKE '%evidence-inspect-%' BEGIN SELECT RAISE(ABORT,'copy-evidence-write-denied'); END`
	if err := f.db.Exec(trigger).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.store.saveCopyEvidence(f.scope, f.expect.OperationID, "inspect", evidence); err == nil || !strings.Contains(err.Error(), "copy-evidence-write-denied") {
		t.Fatal("actual evidence SQL failure was ignored", err)
	}
	after, err := f.store.Read(f.scope, f.expect.OperationID)
	if err != nil || after.State.ControlRevision != before.State.ControlRevision || len(after.Receipt.Execution.Evidence) != 0 {
		t.Fatal("evidence failure partially committed")
	}
	if err := f.db.Exec("DROP TRIGGER reject_copy_evidence").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.store.saveCopyEvidence(f.scope, f.expect.OperationID, "inspect", evidence); err != nil {
		t.Fatal(err)
	}
	after, err = f.store.Read(f.scope, f.expect.OperationID)
	if err != nil || after.Receipt.Execution.Evidence["inspect"].Report == nil || after.Receipt.Execution.Evidence["inspect"].Report.ErrorCode != "copy_io_error" || after.Receipt.Steps["restore-copy-verified"].Applied {
		t.Fatal("full safe failure evidence was lost or counted as verification")
	}
	revision := after.State.ControlRevision
	if err := f.store.saveCopyEvidence(f.scope, f.expect.OperationID, "inspect", evidence); err != nil {
		t.Fatal(err)
	}
	after, err = f.store.Read(f.scope, f.expect.OperationID)
	if err != nil || after.State.ControlRevision != revision {
		t.Fatal("identical evidence replay rewrote SQL")
	}
	evidence.Identity.PodUID = "replacement-pod-uid"
	if err := f.store.saveCopyEvidence(f.scope, f.expect.OperationID, "inspect", evidence); !errors.Is(err, ErrConflict) {
		t.Fatal("evidence adopted another Pod", err)
	}
}

func TestResourceUnavailableOrMalformedRequestNeverReachesCreate(t *testing.T) {
	for _, kind := range []string{"core", "dynamic", "store", "cancelled", "pod", "kind", "namespace", "purpose", "unmarshalable"} {
		t.Run(kind, func(t *testing.T) {
			f, frame, e := resourceFixture(t)
			client := e.Dynamic.(*dynamicfake.FakeDynamicClient)
			ctx := context.Background()
			cm := &corev1.ConfigMap{ObjectMeta: frame.metadata(frame.name(0, "inspect-plan"), "restore-plan")}
			var obj interface{} = cm
			resourceKind, purpose := "ConfigMap", "restore-plan"
			switch kind {
			case "core":
				e.Core = nil
			case "dynamic":
				e.Dynamic = nil
			case "store":
				e.Store = nil
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "pod":
				resourceKind = "Pod"
			case "kind":
				resourceKind = "Deployment"
			case "namespace":
				cm.Namespace = "foreign"
			case "purpose":
				purpose = "invalid/purpose"
			case "unmarshalable":
				obj = func() {}
			}
			if _, err := e.Ensure(ctx, f.scope, f.expect.OperationID, "inspect-plan", purpose, resourceKind, obj); err == nil {
				t.Fatal("unavailable or malformed resource request was accepted")
			}
			for _, action := range client.Actions() {
				if action.GetVerb() == "create" {
					t.Fatal("denied resource request reached Kubernetes Create")
				}
			}
		})
	}
}
