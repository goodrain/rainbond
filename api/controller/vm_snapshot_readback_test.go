package controller

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/goodrain/rainbond/pkg/vmrestore"
)

// capability_id: rainbond.vm-snapshot.authoritative-readback
func TestSnapshotHTTPPreservesActualStoreReadbackAndIgnoresIdentityQueries(t *testing.T) {
	// Captured from the actual Store/Engine lifecycle test after reopening SQL.
	// The facade below is a transport adapter, not a native/backend acceptance.
	raw, err := os.ReadFile("testdata/snapshot_readback_from_store.json")
	if err != nil {
		t.Fatal(err)
	}
	var actual vmrestore.SnapshotOperationDTO
	if err := json.Unmarshal(raw, &actual); err != nil {
		t.Fatal(err)
	}
	service := &snapshotHTTPService{dto: actual}
	router := snapshotHTTPRouter(&VMSnapshotController{service: service})
	response := snapshotHTTPRequest(router, "GET", "/vm-snapshots/a?namespace=foreign&source_vm_name=forged&source_vm_uid=forged&manifest_id=forged", "", true)
	if response.Code != 200 || service.calls != 1 {
		t.Fatal("actual GET contract failed", response.Code)
	}
	var envelope struct {
		Bean vmrestore.SnapshotOperationDTO `json:"bean"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || !reflect.DeepEqual(actual, envelope.Bean) {
		t.Fatal("authoritative DTO changed through HTTP projection", err)
	}
	response = snapshotHTTPRequest(router, "GET", "/vm-snapshots/a", "", false)
	if response.Code != 403 || service.calls != 1 {
		t.Fatal("missing scope reached the raw readback facade")
	}
}

// capability_id: rainbond.vm-snapshot.authoritative-readback
func TestSnapshotLegacyFlagsDoNotSynthesizeReleaseReadback(t *testing.T) {
	var legacy vmrestore.SnapshotOperationDTO
	if err := json.Unmarshal([]byte(`{"manifest_acked":true,"finalized":true}`), &legacy); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"namespace", "source_vm_name", "manifest_id", "manifest_document_hash", "finalized_control_revision", "fence_owned", "fence_cleared"} {
		if _, exists := fields[name]; exists {
			t.Fatal("legacy bools acquired nonexistent proof", name)
		}
	}
}
