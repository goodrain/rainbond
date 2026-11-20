package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/require"
)

type innerPortHTTPService struct {
	handler.ServiceHandler
	err                   error
	calls                 int
	tenant, id, operation string
	port                  int
}

func (s *innerPortHTTPService) PortInner(tenant, serviceID, operation string, port int) error {
	s.calls++
	s.tenant, s.id, s.operation, s.port = tenant, serviceID, operation, port
	return s.err
}

type innerPortHTTPGateway struct {
	handler.GatewayHandler
	err   error
	tasks []map[string]interface{}
}

func (g *innerPortHTTPGateway) SendTaskDeprecated(task map[string]interface{}) error {
	g.tasks = append(g.tasks, task)
	return g.err
}

func innerPortHTTPRequest(operation, port string) *http.Request {
	r := httptest.NewRequest(http.MethodPut, "/ports/"+port+"/inner", strings.NewReader(fmt.Sprintf(`{"operation":%q}`, operation)))
	ctx := context.WithValue(r.Context(), ctxutil.ContextKey("service_id"), "vm-service")
	ctx = context.WithValue(ctx, ctxutil.ContextKey("tenant_name"), "team")
	route := chi.NewRouteContext()
	route.URLParams.Add("port", port)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, route)
	return r.WithContext(ctx)
}

// capability_id: rainbond.service.inner-port-reconciles-idempotent-access
func TestPortInnerControllerReconcilesAccessOnRetry(t *testing.T) {
	for _, tc := range []struct {
		name      string
		operation string
		stateErr  error
	}{
		{name: "new open", operation: "open"},
		{name: "already open", operation: "open", stateErr: errors.New("already open")},
		{name: "new close", operation: "close"},
		{name: "already closed", operation: "close", stateErr: errors.New("already close")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &innerPortHTTPService{err: tc.stateErr}
			gateway := &innerPortHTTPGateway{}
			response := httptest.NewRecorder()

			portInnerControllerWithHandlers(response, innerPortHTTPRequest(tc.operation, "80"), service, gateway)

			require.Equal(t, http.StatusOK, response.Code)
			if tc.stateErr != nil {
				require.Contains(t, response.Body.String(), tc.stateErr.Error())
			}
			require.Equal(t, 1, service.calls)
			require.Equal(t, "team", service.tenant)
			require.Equal(t, "vm-service", service.id)
			require.Equal(t, tc.operation, service.operation)
			require.Equal(t, 80, service.port)
			require.Equal(t, []map[string]interface{}{{
				"service_id": "vm-service", "action": "port-" + tc.operation, "port": 80, "is_inner": true,
			}}, gateway.tasks)
		})
	}
}

// capability_id: rainbond.service.inner-port-surfaces-dispatch-errors
func TestPortInnerControllerReturnsDispatchFailure(t *testing.T) {
	for _, tc := range []struct {
		name      string
		operation string
		stateErr  error
	}{
		{name: "new open", operation: "open"},
		{name: "retry open", operation: "open", stateErr: errors.New("already open")},
		{name: "new close", operation: "close"},
		{name: "retry close", operation: "close", stateErr: errors.New("already close")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &innerPortHTTPService{err: tc.stateErr}
			gateway := &innerPortHTTPGateway{err: errors.New("queue unavailable")}
			response := httptest.NewRecorder()

			portInnerControllerWithHandlers(response, innerPortHTTPRequest(tc.operation, "80"), service, gateway)

			require.Equal(t, http.StatusInternalServerError, response.Code)
			require.Len(t, gateway.tasks, 1)
			require.Contains(t, response.Body.String(), "queue unavailable")
		})
	}
}

func TestPortInnerControllerRejectsInvalidStateBeforeDispatch(t *testing.T) {
	for _, tc := range []struct {
		name       string
		operation  string
		port       string
		stateErr   error
		wantStatus int
		wantCalls  int
	}{
		{name: "missing port", operation: "open", port: "80", stateErr: gorm.ErrRecordNotFound, wantStatus: http.StatusNotFound, wantCalls: 1},
		{name: "database failure", operation: "open", port: "80", stateErr: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantCalls: 1},
		{name: "invalid operation", operation: "unknown", port: "80", wantStatus: http.StatusBadRequest},
		{name: "invalid port", operation: "open", port: "invalid", wantStatus: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &innerPortHTTPService{err: tc.stateErr}
			gateway := &innerPortHTTPGateway{}
			response := httptest.NewRecorder()

			portInnerControllerWithHandlers(response, innerPortHTTPRequest(tc.operation, tc.port), service, gateway)

			require.Equal(t, tc.wantStatus, response.Code)
			require.Equal(t, tc.wantCalls, service.calls)
			require.Empty(t, gateway.tasks)
		})
	}
}
