package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
)

type gatewayConsumerControllerStub struct {
	called string
}

func (s *gatewayConsumerControllerStub) ListGatewayJWTConsumers(w http.ResponseWriter, _ *http.Request) {
	s.called = "list"
	w.WriteHeader(http.StatusNoContent)
}

func (s *gatewayConsumerControllerStub) CreateGatewayJWTConsumer(w http.ResponseWriter, _ *http.Request) {
	s.called = "create"
	w.WriteHeader(http.StatusNoContent)
}

func (s *gatewayConsumerControllerStub) RotateGatewayJWTConsumer(w http.ResponseWriter, _ *http.Request) {
	s.called = "rotate"
	w.WriteHeader(http.StatusNoContent)
}

func (s *gatewayConsumerControllerStub) DeleteGatewayJWTConsumer(w http.ResponseWriter, _ *http.Request) {
	s.called = "delete"
	w.WriteHeader(http.StatusNoContent)
}

func TestGatewayConsumerRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantCalled string
	}{
		{name: "list", method: http.MethodGet, path: "/consumers/", wantCalled: "list"},
		{name: "create", method: http.MethodPost, path: "/consumers/", wantCalled: "create"},
		{name: "rotate", method: http.MethodPost, path: "/consumers/orders/credentials", wantCalled: "rotate"},
		{name: "delete", method: http.MethodDelete, path: "/consumers/orders", wantCalled: "delete"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &gatewayConsumerControllerStub{}
			router := chi.NewRouter()
			router.Route("/consumers", func(r chi.Router) {
				registerGatewayConsumerRoutes(r, stub)
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))

			if recorder.Code != http.StatusNoContent || stub.called != test.wantCalled {
				t.Fatalf("%s %s: status=%d called=%q, want status=%d called=%q", test.method, test.path, recorder.Code, stub.called, http.StatusNoContent, test.wantCalled)
			}
		})
	}
}
