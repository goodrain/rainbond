package handler

import (
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.resource-import.daemonset-component
func TestExtendMethodForResourceTypeSupportsDaemonSet(t *testing.T) {
	got, ok := extendMethodForResourceType(apimodel.DaemonSet)
	if !ok {
		t.Fatalf("expected daemonset resource type to be supported")
	}
	if got != dbmodel.ServiceTypeDaemonSet.String() {
		t.Fatalf("expected daemonset extend method %q, got %q", dbmodel.ServiceTypeDaemonSet.String(), got)
	}
}
