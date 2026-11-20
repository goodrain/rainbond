package vmrestore

import (
	"errors"
	"testing"
)

// capability_id: rainbond.vm-restore.materialized-canonical-identity
func TestMaterializedCanonicalProvenanceUsesAnEmptyPair(t *testing.T) {
	scope := Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	for _, test := range []struct {
		name, operation, snapshot string
		allowed                   bool
	}{{"canonical", "", "", true}, {"snapshot", "operation-a", "snapshot-uid", true}, {"half-operation", "operation-a", "", false}, {"half-snapshot", "", "snapshot-uid", false}} {
		t.Run(test.name, func(t *testing.T) {
			state := NewState(scope)
			state.Bindings["7"] = Binding{VolumeID: 7, DiskKey: "data", CanonicalName: "manual7", Role: "data", Attached: true, ActivePVC: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "cdi-alias", UID: "actual-uid"}, RestoreID: test.operation, SnapshotUID: test.snapshot}
			err := state.Validate(scope)
			if test.allowed && err != nil {
				t.Fatal("verified canonical identity required fabricated snapshot provenance")
			}
			if !test.allowed && !errors.Is(err, ErrCorrupt) {
				t.Fatal("half provenance pair was accepted")
			}
		})
	}
}
