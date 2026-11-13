package vmrestore

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type bindingPlanFixture struct {
	scope   Scope
	expect  Expectation
	intent  RestoreIntent
	old     map[string]Binding
	after   RestoreConfig
	targets map[uint]ObjectRef
}

func newBindingPlanFixture(t *testing.T) bindingPlanFixture {
	t.Helper()
	scope := Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	old := map[string]Binding{}
	config := RestoreConfig{Hardware: SnapshotConfig{CPUMilli: 2000, MemoryMiB: 4096, BootMode: "uefi", DiskLayout: []SnapshotLayout{}}, Volumes: []RestoreVolumeConfig{}}
	targets := map[uint]ObjectRef{}
	for index, id := range []uint{7, 8, 9} {
		key := "data-" + strconv.Itoa(index)
		role := "data"
		if index == 0 {
			key = "disk"
			role = "root"
		}
		old[strconv.FormatUint(uint64(id), 10)] = Binding{VolumeID: id, DiskKey: key, CanonicalName: canonicalVolumeName(id), Role: role, Attached: true, ActivePVC: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: canonicalVolumeName(id), UID: "old-" + strconv.Itoa(index)}}
		if index == 2 {
			continue
		}
		config.Hardware.DiskLayout = append(config.Hardware.DiskLayout, SnapshotLayout{VolumeID: id, DiskKey: key, CanonicalName: canonicalVolumeName(id), Role: role, Device: "disk", Bus: "virtio", BootOrder: uint(index + 1)})
		config.Volumes = append(config.Volumes, RestoreVolumeConfig{VolumeID: id, DiskKey: key, CapacityGiB: 20, StorageClass: "cephfs-vm-snapshot", AccessMode: "ReadWriteMany"})
		targets[id] = ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "target-" + strconv.Itoa(index), UID: "target-uid-" + strconv.Itoa(index)}
	}
	return bindingPlanFixture{scope: scope, expect: Expectation{OperationID: "operation-a"}, intent: RestoreIntent{SourceSnapshot: ObjectRef{Kind: "VirtualMachineSnapshot", Namespace: scope.Namespace, Name: "snapshot-a", UID: "snapshot-a-uid"}}, old: old, after: config, targets: targets}
}

// capability_id: rainbond.vm-restore.materialized-binding-plan
func TestRestoreBindingPlanPreservesLaterBDisksWithoutReattaching(t *testing.T) {
	f := newBindingPlanFixture(t)
	next, err := BuildRestoreBindings(f.scope, f.expect.OperationID, f.intent.SourceSnapshot.UID, f.old, f.after, f.targets)
	if err != nil {
		t.Fatal(err)
	}
	if next["9"].Attached || next["9"].ActivePVC.UID != f.old["9"].ActivePVC.UID || next["7"].PreviousPVC.UID != f.old["7"].ActivePVC.UID || next["7"].ActiveDV != nil {
		t.Fatal("plan lost retained B or fabricated target DV")
	}
	state := NewState(f.scope)
	state.Bindings = next
	if _, mapped, err := state.ResolveClaim(9); !mapped || !errors.Is(err, ErrDetached) {
		t.Fatal("explicitly detached disk fell back to canonical B")
	}
	if !f.old["9"].Attached || f.old["7"].PreviousPVC != nil {
		t.Fatal("planning mutated B rollback identity")
	}
}
func TestRestoreBindingPlanRejectsMissingForeignAndReusedTargets(t *testing.T) {
	for _, kind := range []string{"missing", "same-b", "other-b", "alias", "namespace", "extra", "root-role", "layout"} {
		t.Run(kind, func(t *testing.T) {
			f := newBindingPlanFixture(t)
			switch kind {
			case "missing":
				delete(f.targets, 8)
			case "same-b":
				f.targets[7] = f.old["7"].ActivePVC
			case "other-b":
				f.targets[7] = f.old["9"].ActivePVC
			case "alias":
				f.targets[8] = f.targets[7]
			case "namespace":
				v := f.targets[7]
				v.Namespace = "foreign"
				f.targets[7] = v
			case "extra":
				f.targets[9] = ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: "extra", UID: "extra"}
			case "root-role":
				v := f.old["7"]
				v.Role = "data"
				f.old["7"] = v
			case "layout":
				f.after.Hardware.DiskLayout[1].BootOrder = 1
			}
			if _, err := BuildRestoreBindings(f.scope, f.expect.OperationID, f.intent.SourceSnapshot.UID, f.old, f.after, f.targets); err == nil {
				t.Fatal("unsafe binding plan was accepted")
			}
		})
	}
}
func TestRestoreConfigHashIsOrderStableWithoutMutatingCallers(t *testing.T) {
	f := newBindingPlanFixture(t)
	before := append([]RestoreVolumeConfig(nil), f.after.Volumes...)
	first, err := RestoreConfigHash(f.after)
	if err != nil {
		t.Fatal(err)
	}
	f.after.Volumes[0], f.after.Volumes[1] = f.after.Volumes[1], f.after.Volumes[0]
	second, err := RestoreConfigHash(f.after)
	if err != nil || first != second || !reflect.DeepEqual(before[0], f.after.Volumes[1]) {
		t.Fatal("config identity depended on request ordering or mutated input")
	}
}

func TestRestoreBindingPlanDoesNotAliasOriginalDiskReferences(t *testing.T) {
	for _, readonlyData := range []bool{false, true} {
		t.Run(strconv.FormatBool(readonlyData), func(t *testing.T) {
			f := newBindingPlanFixture(t)
			for _, key := range []string{"7", "8", "9"} {
				b := f.old[key]
				b.ActiveDV = &ObjectRef{Kind: "DataVolume", Namespace: f.scope.Namespace, Name: "dv-" + key, UID: "dv-uid-" + key}
				b.PreviousPVC = &ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: "older-" + key, UID: "older-pvc-" + key}
				b.PreviousDV = &ObjectRef{Kind: "DataVolume", Namespace: f.scope.Namespace, Name: "older-dv-" + key, UID: "older-dv-" + key}
				f.old[key] = b
			}
			if readonlyData {
				f.after.Hardware.DiskLayout[1].ReadOnly = true
				f.after.Volumes[1].ReadOnly = true
				delete(f.targets, 8)
			}
			next, err := BuildRestoreBindings(f.scope, f.expect.OperationID, f.intent.SourceSnapshot.UID, f.old, f.after, f.targets)
			if err != nil {
				t.Fatal(err)
			}
			next["7"].PreviousDV.UID = "changed-result"
			if f.old["7"].ActiveDV.UID != "dv-uid-7" {
				t.Error("planned rollback DV aliases original active identity")
			}
			if readonlyData {
				next["8"].ActiveDV.UID = "changed-readonly-result"
				next["8"].PreviousPVC.UID = "changed-previous-result"
				next["8"].PreviousDV.UID = "changed-previous-dv-result"
				if f.old["8"].ActiveDV.UID != "dv-uid-8" || f.old["8"].PreviousPVC.UID != "older-pvc-8" || f.old["8"].PreviousDV.UID != "older-dv-8" {
					t.Error("readonly disk result aliases original retained identities")
				}
			}
			next["9"].ActiveDV.UID = "changed-detached-result"
			if f.old["9"].ActiveDV.UID != "dv-uid-9" {
				t.Error("detached disk result aliases original identity")
			}
		})
	}
}

func TestRestoreConfigRejectsUnrepresentableHardwareAndDiskSets(t *testing.T) {
	mutations := map[string]func(*RestoreConfig){
		"fractional cpu":         func(c *RestoreConfig) { c.Hardware.CPUMilli = 1500 },
		"missing memory":         func(c *RestoreConfig) { c.Hardware.MemoryMiB = 0 },
		"unknown boot":           func(c *RestoreConfig) { c.Hardware.BootMode = "unknown" },
		"missing layout":         func(c *RestoreConfig) { c.Hardware.DiskLayout = nil },
		"missing volume":         func(c *RestoreConfig) { c.Volumes = c.Volumes[:1] },
		"invalid class":          func(c *RestoreConfig) { c.Volumes[0].StorageClass = "invalid/class" },
		"invalid access":         func(c *RestoreConfig) { c.Volumes[0].AccessMode = "unknown" },
		"zero capacity":          func(c *RestoreConfig) { c.Volumes[0].CapacityGiB = 0 },
		"duplicate volume":       func(c *RestoreConfig) { c.Volumes = append(c.Volumes, c.Volumes[0]) },
		"duplicate device":       func(c *RestoreConfig) { c.Hardware.DiskLayout[1] = c.Hardware.DiskLayout[0] },
		"wrong logical identity": func(c *RestoreConfig) { c.Hardware.DiskLayout[0].DiskKey = "other" },
		"wrong canonical device": func(c *RestoreConfig) { c.Hardware.DiskLayout[0].CanonicalName = "manual99" },
		"unknown device":         func(c *RestoreConfig) { c.Hardware.DiskLayout[0].Device = "unknown" },
		"unknown bus":            func(c *RestoreConfig) { c.Hardware.DiskLayout[0].Bus = "unknown" },
		"readonly root":          func(c *RestoreConfig) { c.Hardware.DiskLayout[0].ReadOnly = true; c.Volumes[0].ReadOnly = true },
		"duplicate boot order":   func(c *RestoreConfig) { c.Hardware.DiskLayout[1].BootOrder = 1 },
		"two roots":              func(c *RestoreConfig) { c.Hardware.DiskLayout[1].Role = "root" },
		"no root":                func(c *RestoreConfig) { c.Hardware.DiskLayout[0].Role = "data" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			f := newBindingPlanFixture(t)
			if err := f.after.validate(f.scope); err != nil {
				t.Fatal("valid control failed")
			}
			mutate(&f.after)
			if !errors.Is(f.after.validate(f.scope), ErrCorrupt) {
				t.Fatal("invalid selected configuration was accepted")
			}
		})
	}
}

func TestRestoreIntentPinsSourceScopeAndHashes(t *testing.T) {
	f := newBindingPlanFixture(t)
	valid := RestoreIntent{CurrentVM: ObjectRef{Kind: "VirtualMachine", Namespace: f.scope.Namespace, Name: "vm-current", UID: "current-uid"}, SourceSnapshot: f.intent.SourceSnapshot, SourceContent: ObjectRef{Kind: "VirtualMachineSnapshotContent", Namespace: f.scope.Namespace, Name: "content-a", UID: "content-uid"}, SourceVMUID: "source-uid", SourceCoreHash: strings.Repeat("a", 64), SourceDocumentHash: strings.Repeat("b", 64), ConfigAfterHash: strings.Repeat("c", 64), PowerAfter: "previous", Before: f.after}
	if err := valid.validate(f.scope); err != nil {
		t.Fatal("valid source proof was rejected")
	}
	for _, kind := range []string{"current namespace", "snapshot namespace", "content namespace", "missing source uid", "invalid core", "invalid document", "invalid after", "unknown power"} {
		t.Run(kind, func(t *testing.T) {
			r := valid
			switch kind {
			case "current namespace":
				r.CurrentVM.Namespace = "foreign"
			case "snapshot namespace":
				r.SourceSnapshot.Namespace = "foreign"
			case "content namespace":
				r.SourceContent.Namespace = "foreign"
			case "missing source uid":
				r.SourceVMUID = ""
			case "invalid core":
				r.SourceCoreHash = "invalid"
			case "invalid document":
				r.SourceDocumentHash = "invalid"
			case "invalid after":
				r.ConfigAfterHash = "invalid"
			case "unknown power":
				r.PowerAfter = "always"
			}
			if !errors.Is(r.validate(f.scope), ErrCorrupt) {
				t.Fatal("invalid source proof was accepted")
			}
		})
	}
	before := valid
	if !sameRestoreIntent(&valid, &before) {
		t.Fatal("identical immutable intent did not match")
	}
	before.SourceSnapshot.UID = "replacement"
	if sameRestoreIntent(&valid, &before) {
		t.Fatal("changed snapshot identity matched original intent")
	}
}

func TestRestoreBindingPlanRejectsContradictoryObjectSlots(t *testing.T) {
	for _, scenario := range []string{"two-targets", "retained-active", "retained-previous", "contradictory-originals"} {
		t.Run(scenario, func(t *testing.T) {
			f := newBindingPlanFixture(t)
			switch scenario {
			case "two-targets":
				target := f.targets[7]
				target.Name = f.targets[8].Name
				f.targets[7] = target
			case "retained-active":
				target := f.targets[7]
				target.Name = f.old["9"].ActivePVC.Name
				f.targets[7] = target
			case "retained-previous":
				b := f.old["9"]
				b.PreviousPVC = &ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: f.targets[7].Name, UID: "retained-previous-uid"}
				f.old["9"] = b
			case "contradictory-originals":
				b := f.old["9"]
				b.ActivePVC.Name = f.old["7"].ActivePVC.Name
				f.old["9"] = b
			}
			before, marshalErr := json.Marshal(f.old)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if _, err := BuildRestoreBindings(f.scope, f.expect.OperationID, f.intent.SourceSnapshot.UID, f.old, f.after, f.targets); !errors.Is(err, ErrConflict) {
				t.Fatal("contradictory namespace/name identity set was accepted")
			}
			after, marshalErr := json.Marshal(f.old)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if string(after) != string(before) {
				t.Fatal("rejected planning changed original rollback bindings")
			}
		})
	}
}

func TestRestoreBindingPlanAllowsDistinctResourceKindsAndSharedHistory(t *testing.T) {
	f := newBindingPlanFixture(t)
	b := f.old["9"]
	b.ActiveDV = &ObjectRef{Kind: "DataVolume", Namespace: f.scope.Namespace, Name: f.targets[7].Name, UID: "retained-dv-uid"}
	b.PreviousPVC = &ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: f.old["7"].ActivePVC.Name, UID: f.old["7"].ActivePVC.UID}
	f.old["9"] = b
	next, err := BuildRestoreBindings(f.scope, f.expect.OperationID, f.intent.SourceSnapshot.UID, f.old, f.after, f.targets)
	if err != nil || next["9"].Attached || next["9"].ActiveDV.UID != "retained-dv-uid" || next["7"].ActivePVC.UID != f.targets[7].UID {
		t.Fatalf("valid kind-separated targets or exact historical refs rejected: %v", err)
	}
}
