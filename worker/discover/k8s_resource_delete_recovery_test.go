// RAINBOND, Application Management Platform
// Copyright (C) 2026 Goodrain Co., Ltd.

package discover

import (
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.k8s-resource.batched-delete-recovery
func TestK8sResourceDeleteRecoveryBuildsOneBoundedBatchTask(t *testing.T) {
	resources := make([]dbmodel.K8sResource, 0, 32)
	for i := 1; i <= 32; i++ {
		resources = append(resources, dbmodel.K8sResource{
			Model:            dbmodel.Model{ID: uint(i)},
			DeleteGeneration: int64(i + 10),
		})
	}

	body := newK8sResourceDeleteRecoveryTask(resources)
	if len(body.Resources) != 32 {
		t.Fatalf("expected one 32-target recovery task, got %d", len(body.Resources))
	}
	if body.Resources[0].ResourceID != 1 || body.Resources[0].DeleteGeneration != 11 {
		t.Fatalf("unexpected first recovery target: %#v", body.Resources[0])
	}
	if body.Resources[31].ResourceID != 32 || body.Resources[31].DeleteGeneration != 42 {
		t.Fatalf("unexpected last recovery target: %#v", body.Resources[31])
	}
}
