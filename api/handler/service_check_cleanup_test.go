package handler

import "testing"

// capability_id: rainbond.cleanup.service_check_import_handoff
func TestServiceCheckCreationCleanupPreservesImportReferences(t *testing.T) {
	for _, value := range []string{
		`{"check_status":"Success","service_info":[{"tar_images":[{"name":"app:v1","prefix":"goodrain.me"}]}]}`,
		`{"check_status":"Failure","service_info":[{"tar_images":[{"name":"app:v1","prefix":"goodrain.me"}]}]}`,
		`{"check_status":"Success","service_info":[{"tar_images":[]}]}`,
		`{"check_status":"Running"}`, `invalid`, `null`, `{}`,
	} {
		if serviceCheckCanBeReleasedOnCreate(value) {
			t.Fatal("creation released pending or unverified import receipt")
		}
	}
	for _, value := range []string{
		`{"check_status":"Success","service_info":[{"image":"example/app:v1"}]}`,
		`{"check_status":"Failure","service_info":[]}`,
	} {
		if !serviceCheckCanBeReleasedOnCreate(value) {
			t.Fatal("ordinary terminal check retained")
		}
	}
}
