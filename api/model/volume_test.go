package model

import (
	"encoding/json"
	"testing"
)

// capability_id: rainbond.component.volume-update-preserves-capacity
func TestUpdVolumeReqPreservesVolumeCapacityFromJSON(t *testing.T) {
	raw := []byte(`{
		"volume_name":"data",
		"volume_type":"share-file",
		"volume_path":"/data",
		"volume_capacity":20
	}`)

	var req UpdVolumeReq
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal update volume request: %v", err)
	}

	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal update volume request: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded payload: %v", err)
	}

	got, ok := payload["volume_capacity"]
	if !ok {
		t.Fatalf("expected volume_capacity to be preserved in request payload, got %s", string(encoded))
	}
	if got != float64(20) {
		t.Fatalf("expected volume_capacity 20, got %#v", got)
	}
}
