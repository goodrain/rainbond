// RAINBOND, Application Management Platform
// Copyright (C) 2026 Goodrain Co., Ltd.

package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.k8s-resource.delete-status-contract
func TestK8sResourceDeleteStatusDoesNotExposePersistedYAML(t *testing.T) {
	status := newK8sResourceDeleteStatus(dbmodel.K8sResource{
		Model:            dbmodel.Model{ID: 42},
		Name:             "example-config",
		Kind:             "ConfigMap",
		Content:          "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: secret-content",
		DeleteStatus:     dbmodel.K8sResourceDeleteStatusDeleting,
		DeleteGeneration: 3,
	})

	body, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal delete status: %v", err)
	}
	if strings.Contains(string(body), "secret-content") || strings.Contains(string(body), "content") || strings.Contains(string(body), "resource_yaml") {
		t.Fatalf("delete status must not expose saved YAML, got %s", body)
	}
	if status.ResourceID != 42 || status.DeleteGeneration != 3 || status.DeleteStatus != model.K8sResourceDeleteStatusDeleting {
		t.Fatalf("unexpected identity/status response: %#v", status)
	}
}
