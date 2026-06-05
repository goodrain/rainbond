package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

type wrapELTestManager struct {
	db.Manager
	eventDao dbdao.EventDao
}

func (m wrapELTestManager) ServiceEventDao() dbdao.EventDao {
	return m.eventDao
}

type wrapELTestEventDao struct {
	dbdao.EventDao
	added int
}

func (d *wrapELTestEventDao) GetLastASyncEvent(target, targetID string) (*dbmodel.ServiceEvent, error) {
	return nil, gorm.ErrRecordNotFound
}

func (d *wrapELTestEventDao) AddModel(arg dbmodel.Interface) error {
	d.added++
	return nil
}

// capability_id: rainbond.vm-live-update.running-shrink-restart-allowed-before-event
func TestWrapELAllowsRunningVMShrinkToReachHandler(t *testing.T) {
	eventDao := &wrapELTestEventDao{}
	db.SetTestManager(wrapELTestManager{eventDao: eventDao})
	defer db.SetTestManager(nil)

	testCases := []struct {
		name string
		body string
	}{
		{
			name: "cpu_shrink",
			body: `{"container_cpu":4000,"container_memory":12288}`,
		},
		{
			name: "memory_shrink",
			body: `{"container_cpu":6000,"container_memory":8192}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := WrapEL(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}, dbmodel.TargetTypeService, "vertical-service", dbmodel.ASYNEVENTTYPE, false)

			req := httptest.NewRequest(http.MethodPut, "/v2/tenants/demo/services/service-vm/vertical", bytes.NewBufferString(tc.body))
			ctx := context.WithValue(req.Context(), ctxutil.ContextKey("tenant_id"), "tenant-1")
			ctx = context.WithValue(ctx, ctxutil.ContextKey("service_id"), "service-vm")
			ctx = context.WithValue(ctx, ctxutil.ContextKey("service"), &dbmodel.TenantServices{
				ServiceID:       "service-vm",
				ExtendMethod:    "vm",
				ContainerCPU:    6000,
				ContainerMemory: 12288,
			})
			req = req.WithContext(ctx)

			resp := httptest.NewRecorder()
			handler(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.Code)
			}
			if !called {
				t.Fatal("expected request to reach wrapped handler")
			}
		})
	}

	if eventDao.added != len(testCases) {
		t.Fatalf("expected one event per allowed vm shrink request, got %d", eventDao.added)
	}
}
