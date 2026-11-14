package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
)

type restoreHTTPService struct {
	calls int
	err   error
	dto   vmrestore.RestoreOperationDTO
}

func (s *restoreHTTPService) GetVMRestoreControl(context.Context, string) (vmrestore.RestoreControlDTO, error) {
	s.calls++
	return vmrestore.RestoreControlDTO{}, s.err
}

func (s *restoreHTTPService) GetVMRestore(context.Context, string, string) (vmrestore.RestoreOperationDTO, error) {
	s.calls++
	return s.dto, s.err
}
func (s *restoreHTTPService) VMRestoreCommand(context.Context, string, string, string, vmrestore.RestoreCommand) (vmrestore.RestoreOperationDTO, error) {
	s.calls++
	return s.dto, s.err
}
func restoreHTTPRouter(c *VMRestoreController) http.Handler {
	r := chi.NewRouter()
	r.Get("/vm-restores/{operation_id}", c.GetVMRestore)
	r.Get("/vm-restore-control", c.GetVMRestoreControl)
	r.Post("/vm-restores/{operation_id}/steps/{restore_action}", c.VMRestoreCommand)
	return r
}

// capability_id: rainbond.vm-restore.rpc-http
func TestRestoreHTTPRejectsScopeAndCallerProofInjection(t *testing.T) {
	s := &restoreHTTPService{}
	router := restoreHTTPRouter(&VMRestoreController{service: s})
	for _, body := range []string{`{"namespace":"foreign"}`, `{"complete":true}`, `{"config_committed":true}`, `{"pvc_name":"foreign"}`, `{} {}`, `[]`, `{"mode":"` + strings.Repeat("x", 17000) + `"}`} {
		w := snapshotHTTPRequest(router, "POST", "/vm-restores/op/steps/halt", body, true)
		if w.Code != 400 {
			t.Fatal("unsafe body accepted", w.Code)
		}
	}
	for _, method := range []string{"GET", "POST"} {
		path := "/vm-restores/op"
		if method == "POST" {
			path += "/steps/halt"
		}
		if w := snapshotHTTPRequest(router, method, path, `{}`, false); w.Code != 403 {
			t.Fatal(w.Code)
		}
	}
	if s.calls != 0 {
		t.Fatal("invalid request reached production facade")
	}
}

// capability_id: rainbond.vm-restore.rpc-http
func TestRestoreHTTPAcceptanceAndSafeErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{vmrestore.ErrRestorePending, 202, "vm_restore_pending"}, {vmrestore.ErrRestoreUnknown, 202, "vm_restore_outcome_unknown"}, {vmrestore.ErrRestoreCapability, 422, "vm_restore_provider_not_qualified"}, {restoreplan.ErrCapacity, 422, "vm_restore_capacity_insufficient"}, {vmrestore.ErrInternalAttribute, 403, "vm_restore_forbidden"}, {vmrestore.ErrNotFound, 404, "vm_restore_not_found"}, {vmrestore.ErrConflict, 409, "vm_restore_conflict"}, {vmrestore.ErrReferenceUnavailable, 503, "vm_restore_dependency_unavailable"}, {errors.New("INTERNAL_TEXT_SENTINEL"), 500, "vm_restore_internal_error"},
	} {
		s := &restoreHTTPService{err: tc.err}
		w := snapshotHTTPRequest(restoreHTTPRouter(&VMRestoreController{service: s}), "POST", "/vm-restores/op/steps/halt", `{}`, true)
		body := w.Body.String()
		if w.Code != tc.status || !strings.Contains(body, tc.code) || !strings.Contains(body, `"complete":false`) || strings.Contains(body, "INTERNAL_TEXT_SENTINEL") {
			t.Fatal("unsafe domain response", w.Code, body)
		}
	}
	s := &restoreHTTPService{dto: vmrestore.RestoreOperationDTO{OperationID: "op", Phase: "prepared"}}
	r := restoreHTTPRouter(&VMRestoreController{service: s})
	if w := snapshotHTTPRequest(r, "POST", "/vm-restores/op/steps/cancel", `{}`, true); w.Code != 202 || !strings.Contains(w.Body.String(), `"complete":false`) {
		t.Fatal("acceptance became completion")
	}
	if w := snapshotHTTPRequest(r, "GET", "/vm-restores/op", ``, true); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := snapshotHTTPRequest(r, "GET", "/vm-restore-control", ``, true); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := snapshotHTTPRequest(r, "GET", "/vm-restore-control", ``, false); w.Code != 403 {
		t.Fatal(w.Code)
	}
}

// capability_id: rainbond.vm-restore.rpc-http
func TestExistingPowerHTTPPreservesPendingConflictAndSafeDependencyErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{&bcode.Code{Status: 202, Message: "vm_power_pending"}, 202}, {&bcode.Code{Status: 409, Message: "vm_restore_busy"}, 409}, {vmrestore.ErrRestoreUnknown, 202}, {vmrestore.ErrReferenceUnavailable, 503}, {vmrestore.ErrInternalAttribute, 403}} {
		w := httptest.NewRecorder()
		if !writeRestorePowerError(w, httptest.NewRequest("POST", "/start", nil), tc.err) || w.Code != tc.status || !strings.Contains(w.Body.String(), `"complete":false`) {
			t.Fatal("ordinary power lost restore status", w.Code)
		}
	}
	if writeRestorePowerError(httptest.NewRecorder(), httptest.NewRequest("POST", "/start", nil), errors.New("legacy-native-error")) {
		t.Fatal("unrelated native error handling changed")
	}
}
