package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/require"
)

type rollbackEventStore struct {
	dao.EventDao
	event   *dbmodel.ServiceEvent
	updates int
}

func (s *rollbackEventStore) GetLastASyncEvent(string, string) (*dbmodel.ServiceEvent, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *rollbackEventStore) AddModel(item dbmodel.Interface) error {
	s.event = item.(*dbmodel.ServiceEvent)
	return nil
}

func (s *rollbackEventStore) GetEventByEventID(string) (*dbmodel.ServiceEvent, error) {
	return s.event, nil
}

func (s *rollbackEventStore) UpdateModel(item dbmodel.Interface) error {
	s.updates++
	s.event = item.(*dbmodel.ServiceEvent)
	return nil
}

type rollbackEventManager struct {
	db.Manager
	events *rollbackEventStore
}

func (m rollbackEventManager) ServiceEventDao() dao.EventDao { return m.events }

func serveRollbackResult(t *testing.T, result handler.OperationResult) (*httptest.ResponseRecorder, *rollbackEventStore) {
	t.Helper()
	previous := db.GetManager()
	store := &rollbackEventStore{}
	db.SetTestManager(rollbackEventManager{events: store})
	t.Cleanup(func() { db.SetTestManager(previous) })
	r := httptest.NewRequest(http.MethodPut, "/rollback", strings.NewReader(`{"upgrade_version":"old"}`))
	ctx := context.WithValue(r.Context(), ctxutil.ContextKey("tenant_id"), "tenant")
	ctx = context.WithValue(ctx, ctxutil.ContextKey("service_id"), "service")
	w := httptest.NewRecorder()
	middleware.WrapEL(func(w http.ResponseWriter, r *http.Request) {
		writeRollbackResult(w, r, result)
	}, dbmodel.TargetTypeService, "rollback-service", dbmodel.ASYNEVENTTYPE, false)(w, r.WithContext(ctx))
	return w, store
}

// capability_id: rainbond.rollback.failed-http-status
func TestRollbackResponseEndsFailedAsyncOperation(t *testing.T) {
	for _, status := range []string{"failure", "", "unexpected"} {
		t.Run(status, func(t *testing.T) {
			w, store := serveRollbackResult(t, handler.OperationResult{Status: status, ErrMsg: "rollback selection failed"})
			require.Equal(t, http.StatusInternalServerError, w.Code,
				"WrapEL finishes an async event only when the response is at least400")
			require.Contains(t, w.Body.String(), "rollback selection failed")
			require.Equal(t, 1, store.updates)
			require.Equal(t, "failure", store.event.Status)
			require.Equal(t, "complete", store.event.FinalStatus)
			require.NotEmpty(t, store.event.EndTime)
		})
	}
}

func TestRollbackResponseKeepsSuccessfulAsyncSubmission(t *testing.T) {
	w, store := serveRollbackResult(t, handler.OperationResult{Status: "success", ServiceID: "service",
		EventID: "event", Operation: "rollback"})
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, strings.Contains(w.Body.String(), `"event_id":"event"`))
	require.Contains(t, w.Body.String(), `"status":"success"`)
	require.Zero(t, store.updates, "worker must complete successfully submitted async events")
	require.Empty(t, store.event.FinalStatus)
	require.Empty(t, store.event.EndTime)
}
