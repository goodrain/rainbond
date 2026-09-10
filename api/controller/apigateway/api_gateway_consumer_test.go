package apigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	apisixfake "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned/fake"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	apimodel "github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type gatewayJWTConsumerHandlerStub struct {
	handler.APIGatewayHandler

	configureCalls int
	namespace      string
	appID          string
	auth           *apimodel.ManagedJWTAuthentication
	createCalls    int
	createErr      error
	listCalls      int
	rotateCalls    int
	deleteCalls    int
	lastName       string
	lastNamespace  string
	lastAppID      string
	listResult     []*apimodel.GatewayJWTConsumer
	rotateResult   *apimodel.GatewayJWTConsumer
}

func (s *gatewayJWTConsumerHandlerStub) ConfigureManagedJWTAuth(_ context.Context, namespace, appID string, auth *apimodel.ManagedJWTAuthentication, plugins []v2.ApisixRoutePlugin) ([]v2.ApisixRoutePlugin, error) {
	s.configureCalls++
	s.namespace = namespace
	s.appID = appID
	s.auth = auth
	return handler.BuildManagedJWTPlugins(namespace, auth.ConsumerNames, auth.Config, plugins), nil
}

func (s *gatewayJWTConsumerHandlerStub) CreateGatewayJWTConsumer(_ context.Context, _, _ string, _ *apimodel.GatewayJWTConsumerRequest) (*apimodel.GatewayJWTConsumer, error) {
	s.createCalls++
	return nil, s.createErr
}

func (s *gatewayJWTConsumerHandlerStub) ListGatewayJWTConsumers(_ context.Context, namespace, appID string) ([]*apimodel.GatewayJWTConsumer, error) {
	s.listCalls++
	s.lastNamespace = namespace
	s.lastAppID = appID
	return s.listResult, nil
}

func (s *gatewayJWTConsumerHandlerStub) RotateGatewayJWTConsumer(_ context.Context, namespace, name string, _ *apimodel.GatewayJWTConsumerCredential) (*apimodel.GatewayJWTConsumer, error) {
	s.rotateCalls++
	s.lastNamespace = namespace
	s.lastName = name
	return s.rotateResult, nil
}

func (s *gatewayJWTConsumerHandlerStub) DeleteGatewayJWTConsumer(_ context.Context, namespace, name string) error {
	s.deleteCalls++
	s.lastNamespace = namespace
	s.lastName = name
	return nil
}

// capability_id: rainbond.gateway.jwt-consumer-api
func TestGatewayHTTPRouteRequestBackwardCompatibility(t *testing.T) {
	legacyJSON := []byte(`{
		"name":"legacy-route",
		"priority":9,
		"match":{"hosts":["legacy.example.com"],"paths":["/*"]},
		"plugins":[{"name":"jwt-auth","enable":true,"config":{"header":"authorization"},"secretRef":""}],
		"websocket":true
	}`)

	var request apimodel.GatewayHTTPRouteRequest
	if err := json.Unmarshal(legacyJSON, &request); err != nil {
		t.Fatalf("decode legacy HTTP route request: %v", err)
	}
	if request.ManagedJWTAuth != nil {
		t.Fatalf("ManagedJWTAuth = %#v, want nil for legacy request", request.ManagedJWTAuth)
	}
	if request.Name != "legacy-route" || request.Priority != 9 || !request.Websocket {
		t.Fatalf("decoded legacy route = %#v", request.ApisixRouteHTTP)
	}
	if len(request.Plugins) != 1 || request.Plugins[0].Name != "jwt-auth" {
		t.Fatalf("legacy plugins = %#v, want unchanged jwt-auth plugin", request.Plugins)
	}

	stub := &gatewayJWTConsumerHandlerStub{}
	labels := map[string]string{"creator": "Rainbond"}
	wantPlugins := append([]v2.ApisixRoutePlugin(nil), request.Plugins...)
	if err := configureManagedJWTHTTPRoute(context.Background(), stub, "team-a", "app-1", &request, labels); err != nil {
		t.Fatalf("configure legacy request: %v", err)
	}
	if stub.configureCalls != 0 {
		t.Fatalf("ConfigureManagedJWTAuth calls = %d, want 0", stub.configureCalls)
	}
	if !reflect.DeepEqual(request.Plugins, wantPlugins) {
		t.Fatalf("legacy plugins = %#v, want %#v", request.Plugins, wantPlugins)
	}
	if _, ok := labels[gatewayJWTManagedRouteLabel]; ok {
		t.Fatalf("legacy request added managed label: %#v", labels)
	}

	existingPlugins := handler.BuildManagedJWTPlugins(
		"team-a",
		[]string{"orders"},
		v2.ApisixRoutePluginConfig{"header": "x-managed-jwt"},
		nil,
	)
	existingRoute := &v2.ApisixRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "team-a",
			Labels: map[string]string{
				gatewayJWTManagedRouteLabel: gatewayJWTManagedRouteValue,
			},
		},
		Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{Plugins: existingPlugins}}},
	}
	preserveLegacyManagedJWTHTTPRoute(&request, labels, existingRoute)
	if labels[gatewayJWTManagedRouteLabel] != gatewayJWTManagedRouteValue {
		t.Fatalf("legacy update did not preserve managed marker: %#v", labels)
	}
	if !reflect.DeepEqual(request.Plugins, existingPlugins) {
		t.Fatalf("legacy update plugins = %#v, want observed managed pair %#v", request.Plugins, existingPlugins)
	}
}

func TestPreserveManagedJWTHTTPRouteLabelBeforeCreate(t *testing.T) {
	tests := []struct {
		name        string
		oldRoute    *v2.ApisixRoute
		request     *apimodel.GatewayHTTPRouteRequest
		wantManaged bool
	}{
		{
			name: "marked old route is inherited by renamed route",
			oldRoute: &v2.ApisixRoute{ObjectMeta: metav1.ObjectMeta{
				Name:      "orders-route",
				Namespace: "team-a",
				Labels: map[string]string{
					gatewayJWTManagedRouteLabel: gatewayJWTManagedRouteValue,
				},
			}, Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{
				Plugins: handler.BuildManagedJWTPlugins("team-a", []string{"orders"}, v2.ApisixRoutePluginConfig{"header": "authorization"}, nil),
			}}}},
			request:     &apimodel.GatewayHTTPRouteRequest{},
			wantManaged: true,
		},
		{
			name: "unmarked old route stays unmarked",
			oldRoute: &v2.ApisixRoute{ObjectMeta: metav1.ObjectMeta{
				Name:      "orders-route",
				Namespace: "team-a",
			}},
			request: &apimodel.GatewayHTTPRouteRequest{},
		},
		{
			name:    "missing old route stays unmarked",
			request: &apimodel.GatewayHTTPRouteRequest{},
		},
		{
			name: "explicit managed configuration does not inherit old state",
			oldRoute: &v2.ApisixRoute{ObjectMeta: metav1.ObjectMeta{
				Name:      "orders-route",
				Namespace: "team-a",
				Labels: map[string]string{
					gatewayJWTManagedRouteLabel: gatewayJWTManagedRouteValue,
				},
			}},
			request: &apimodel.GatewayHTTPRouteRequest{
				ManagedJWTAuth: &apimodel.ManagedJWTAuthentication{Enabled: false},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := apisixfake.NewSimpleClientset()
			if test.oldRoute != nil {
				client = apisixfake.NewSimpleClientset(test.oldRoute)
			}
			labels := map[string]string{"creator": "Rainbond"}
			routes := client.ApisixV2().ApisixRoutes("team-a")
			if err := preserveManagedJWTHTTPRouteLabelBeforeCreate(
				context.Background(),
				routes,
				test.request,
				labels,
				"123orders-route-edit",
			); err != nil {
				t.Fatalf("preserve managed route label: %v", err)
			}
			_, gotManaged := labels[gatewayJWTManagedRouteLabel]
			if gotManaged != test.wantManaged {
				t.Fatalf("managed label present = %v, want %v; labels=%#v", gotManaged, test.wantManaged, labels)
			}
			if test.wantManaged {
				gotNames := make([]string, 0, len(test.request.Plugins))
				for _, plugin := range test.request.Plugins {
					gotNames = append(gotNames, plugin.Name)
				}
				if !reflect.DeepEqual(gotNames, []string{"jwt-auth", "consumer-restriction"}) {
					t.Fatalf("renamed route plugins = %#v, want managed pair", gotNames)
				}
			}
		})
	}
}

func TestManagedJWTHTTPRouteRequest(t *testing.T) {
	stub := &gatewayJWTConsumerHandlerStub{}
	request := &apimodel.GatewayHTTPRouteRequest{
		ApisixRouteHTTP: v2.ApisixRouteHTTP{Plugins: []v2.ApisixRoutePlugin{{
			Name:   "proxy-rewrite",
			Enable: true,
			Config: v2.ApisixRoutePluginConfig{"uri": "/v1"},
		}}},
		ManagedJWTAuth: &apimodel.ManagedJWTAuthentication{
			Enabled:       true,
			ConsumerNames: []string{"orders"},
			Config:        v2.ApisixRoutePluginConfig{"header": "authorization"},
		},
	}
	labels := map[string]string{}

	if err := configureManagedJWTHTTPRoute(context.Background(), stub, "team-a", "app-42", request, labels); err != nil {
		t.Fatalf("configure managed JWT route: %v", err)
	}
	if stub.configureCalls != 1 || stub.namespace != "team-a" || stub.appID != "app-42" || stub.auth != request.ManagedJWTAuth {
		t.Fatalf("ConfigureManagedJWTAuth call = calls:%d namespace:%q appID:%q auth:%#v", stub.configureCalls, stub.namespace, stub.appID, stub.auth)
	}
	if labels[gatewayJWTManagedRouteLabel] != gatewayJWTManagedRouteValue {
		t.Fatalf("managed labels = %#v, want ownership marker", labels)
	}
	wantPluginNames := []string{"proxy-rewrite", "jwt-auth", "consumer-restriction"}
	gotPluginNames := make([]string, 0, len(request.Plugins))
	for _, plugin := range request.Plugins {
		gotPluginNames = append(gotPluginNames, plugin.Name)
	}
	if !reflect.DeepEqual(gotPluginNames, wantPluginNames) {
		t.Fatalf("managed plugin names = %#v, want %#v", gotPluginNames, wantPluginNames)
	}

	request.ManagedJWTAuth = &apimodel.ManagedJWTAuthentication{Enabled: false}
	if err := configureManagedJWTHTTPRoute(context.Background(), stub, "team-a", "app-42", request, labels); err != nil {
		t.Fatalf("disable managed JWT route: %v", err)
	}
	if _, ok := labels[gatewayJWTManagedRouteLabel]; ok {
		t.Fatalf("disabled managed route retained ownership marker: %#v", labels)
	}
}

func TestManagedJWTHTTPRouteResponse(t *testing.T) {
	plugins := handler.BuildManagedJWTPlugins(
		"team-a",
		[]string{"orders"},
		v2.ApisixRoutePluginConfig{"header": "authorization"},
		nil,
	)
	marked := &v2.ApisixRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orders-route",
			Namespace: "team-a",
			Labels: map[string]string{
				"app_id":                    "app-42",
				gatewayJWTManagedRouteLabel: gatewayJWTManagedRouteValue,
			},
		},
		Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{
			Name:    "rule",
			Plugins: plugins,
		}}},
	}
	managedResponse := gatewayHTTPRouteResponseFrom(marked)
	if managedResponse.ManagedJWTAuth == nil || !managedResponse.ManagedJWTAuth.Enabled {
		t.Fatalf("managedJwtAuth = %#v, want enabled metadata", managedResponse.ManagedJWTAuth)
	}
	if !reflect.DeepEqual(managedResponse.ManagedJWTAuth.ConsumerNames, []string{"orders"}) {
		t.Fatalf("consumerNames = %#v, want orders", managedResponse.ManagedJWTAuth.ConsumerNames)
	}

	legacy := marked.DeepCopy()
	delete(legacy.Labels, gatewayJWTManagedRouteLabel)
	legacyResponse := gatewayHTTPRouteResponseFrom(legacy)
	if legacyResponse.ManagedJWTAuth != nil {
		t.Fatalf("legacy managedJwtAuth = %#v, want nil", legacyResponse.ManagedJWTAuth)
	}
	encoded, err := json.Marshal(legacyResponse)
	if err != nil {
		t.Fatalf("marshal legacy route response: %v", err)
	}
	if bytes.Contains(encoded, []byte("managedJwtAuth")) {
		t.Fatalf("legacy response exposes managedJwtAuth: %s", encoded)
	}
}

func TestPreserveLegacyManagedJWTHTTPRouteDoesNotAlterUnmarkedRoute(t *testing.T) {
	requestPlugins := []v2.ApisixRoutePlugin{
		{Name: "proxy-rewrite", Enable: true, Config: v2.ApisixRoutePluginConfig{"uri": "/client"}},
		{Name: "jwt-auth", Enable: true, Config: v2.ApisixRoutePluginConfig{"header": "client-copy"}},
	}
	request := &apimodel.GatewayHTTPRouteRequest{ApisixRouteHTTP: v2.ApisixRouteHTTP{Plugins: requestPlugins}}
	oldRoute := &v2.ApisixRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"},
		Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{
			Plugins: handler.BuildManagedJWTPlugins("team-a", []string{"orders"}, nil, nil),
		}}},
	}
	labels := map[string]string{"creator": "Rainbond"}

	preserveLegacyManagedJWTHTTPRoute(request, labels, oldRoute)

	if !reflect.DeepEqual(request.Plugins, requestPlugins) {
		t.Fatalf("unmarked route changed request plugins: got %#v want %#v", request.Plugins, requestPlugins)
	}
	if _, exists := labels[gatewayJWTManagedRouteLabel]; exists {
		t.Fatalf("unmarked route added managed label: %#v", labels)
	}
}

func TestGatewayHTTPRouteValidationRejectsEmptyMatch(t *testing.T) {
	tests := []struct {
		name  string
		route v2.ApisixRouteHTTP
	}{
		{name: "missing hosts", route: v2.ApisixRouteHTTP{Match: v2.ApisixRouteHTTPMatch{Paths: []string{"/*"}}}},
		{name: "missing paths", route: v2.ApisixRouteHTTP{Match: v2.ApisixRouteHTTPMatch{Hosts: []string{"example.com"}}}},
		{name: "blank host", route: v2.ApisixRouteHTTP{Match: v2.ApisixRouteHTTPMatch{Hosts: []string{" "}, Paths: []string{"/*"}}}},
		{name: "blank path", route: v2.ApisixRouteHTTP{Match: v2.ApisixRouteHTTPMatch{Hosts: []string{"example.com"}, Paths: []string{" "}}}},
		{name: "blank additional host", route: v2.ApisixRouteHTTP{Match: v2.ApisixRouteHTTPMatch{Hosts: []string{"example.com", " "}, Paths: []string{"/*"}}}},
		{name: "blank additional path", route: v2.ApisixRouteHTTP{Match: v2.ApisixRouteHTTPMatch{Hosts: []string{"example.com"}, Paths: []string{"/*", " "}}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateGatewayHTTPRouteRequest(&apimodel.GatewayHTTPRouteRequest{ApisixRouteHTTP: test.route}); err == nil {
				t.Fatal("validateGatewayHTTPRouteRequest() error = nil")
			}
		})
	}
}

func TestCreateHTTPAPIRouteRejectsEmptyHostsAndPaths(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty hosts", body: `{"match":{"hosts":[],"paths":["/*"]}}`},
		{name: "empty paths", body: `{"match":{"hosts":["example.com"],"paths":[]}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("tenant"), &dbmodel.Tenants{Namespace: "team-a"}))

			Struct{}.CreateHTTPAPIRoute(recorder, req)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}
}

func TestGatewayHTTPRouteResponsesSkipMissingHTTP(t *testing.T) {
	routes := []v2.ApisixRoute{
		{ObjectMeta: metav1.ObjectMeta{Name: "malformed", Namespace: "team-a"}},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "valid", Namespace: "team-a"},
			Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{
				Match: v2.ApisixRouteHTTPMatch{Hosts: []string{"example.com"}, Paths: []string{"/*"}},
			}}},
		},
	}

	if response := gatewayHTTPRouteResponseFrom(&routes[0]); response != nil {
		t.Fatalf("malformed route response = %#v, want nil", response)
	}
	responses := gatewayHTTPRouteResponses(routes)
	if len(responses) != 1 || responses[0].Name != "|valid|" {
		t.Fatalf("route responses = %#v, want only valid route", responses)
	}
}

func TestReplaceGatewayHTTPRouteRuleHandlesMissingHTTP(t *testing.T) {
	route := &v2.ApisixRoute{}
	want := v2.ApisixRouteHTTP{Name: "replacement"}
	replaceGatewayHTTPRouteRule(route, want)
	if len(route.Spec.HTTP) != 1 || !reflect.DeepEqual(route.Spec.HTTP[0], want) {
		t.Fatalf("HTTP rules = %#v, want %#v", route.Spec.HTTP, []v2.ApisixRouteHTTP{want})
	}
}

func TestGatewayJWTConsumerErrorStatus(t *testing.T) {
	resource := schema.GroupResource{Group: "apisix.apache.org", Resource: "apisixconsumers"}
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid credential", err: fmt.Errorf("wrapped: %w", handler.ErrGatewayJWTConsumerInvalidCredential), want: http.StatusBadRequest},
		{name: "invalid managed auth config", err: handler.ErrGatewayJWTAuthInvalidConfig, want: http.StatusBadRequest},
		{name: "consumer required", err: handler.ErrGatewayJWTConsumerRequired, want: http.StatusBadRequest},
		{name: "external resource", err: handler.ErrGatewayJWTConsumerNotManaged, want: http.StatusForbidden},
		{name: "application mismatch", err: handler.ErrGatewayJWTConsumerAppMismatch, want: http.StatusForbidden},
		{name: "not found", err: fmt.Errorf("wrapped: %w", k8serrors.NewNotFound(resource, "orders")), want: http.StatusNotFound},
		{name: "already exists", err: fmt.Errorf("wrapped: %w", k8serrors.NewAlreadyExists(resource, "orders")), want: http.StatusConflict},
		{name: "in use", err: handler.ErrGatewayJWTConsumerInUse, want: http.StatusConflict},
		{name: "unknown", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := gatewayJWTConsumerErrorStatus(test.err); got != test.want {
				t.Fatalf("gatewayJWTConsumerErrorStatus(%v) = %d, want %d", test.err, got, test.want)
			}
		})
	}
}

func TestCreateGatewayJWTConsumerErrorDoesNotExposeCredential(t *testing.T) {
	const credential = "credential-must-not-leak"
	stub := &gatewayJWTConsumerHandlerStub{
		createErr: fmt.Errorf("%w: rejected %s", handler.ErrGatewayJWTConsumerInvalidCredential, credential),
	}
	original := getAPIGatewayHandler
	getAPIGatewayHandler = func() handler.APIGatewayHandler { return stub }
	t.Cleanup(func() { getAPIGatewayHandler = original })

	body := strings.NewReader(`{"name":"orders","key":"orders-client","secret":"` + credential + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/?appID=app-42", body)
	req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("tenant"), &dbmodel.Tenants{Namespace: "team-a"}))
	recorder := httptest.NewRecorder()

	Struct{}.CreateGatewayJWTConsumer(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if stub.createCalls != 1 {
		t.Fatalf("CreateGatewayJWTConsumer calls = %d, want 1", stub.createCalls)
	}
	if strings.Contains(recorder.Body.String(), credential) {
		t.Fatalf("response exposed credential: %s", recorder.Body.String())
	}
}

func TestGatewayJWTConsumerErrorStatusMapsKubernetesForbidden(t *testing.T) {
	err := k8serrors.NewForbidden(
		schema.GroupResource{Group: "", Resource: "secrets"},
		"rbd-jwt-orders",
		errors.New("denied"),
	)
	if got := gatewayJWTConsumerErrorStatus(err); got != http.StatusForbidden {
		t.Fatalf("gatewayJWTConsumerErrorStatus(forbidden) = %d, want %d", got, http.StatusForbidden)
	}
}

func TestGatewayJWTConsumerControllersForwardRequests(t *testing.T) {
	stub := &gatewayJWTConsumerHandlerStub{
		listResult:   []*apimodel.GatewayJWTConsumer{{Name: "orders"}},
		rotateResult: &apimodel.GatewayJWTConsumer{Name: "orders"},
	}
	original := getAPIGatewayHandler
	getAPIGatewayHandler = func() handler.APIGatewayHandler { return stub }
	t.Cleanup(func() { getAPIGatewayHandler = original })

	withTenant := func(req *http.Request) *http.Request {
		return req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("tenant"), &dbmodel.Tenants{Namespace: "team-a"}))
	}
	withName := func(req *http.Request, name string) *http.Request {
		routeContext := chi.NewRouteContext()
		routeContext.URLParams.Add("name", name)
		return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
	}

	t.Run("list", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		Struct{}.ListGatewayJWTConsumers(recorder, withTenant(httptest.NewRequest(http.MethodGet, "/?appID=app-42", nil)))
		if recorder.Code != http.StatusOK || stub.listCalls != 1 || stub.lastNamespace != "team-a" || stub.lastAppID != "app-42" {
			t.Fatalf("list forwarding: status=%d calls=%d namespace=%q appID=%q", recorder.Code, stub.listCalls, stub.lastNamespace, stub.lastAppID)
		}
	})

	t.Run("rotate", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := withTenant(httptest.NewRequest(http.MethodPost, "/orders/credentials", strings.NewReader(`{"key":"orders-client"}`)))
		req = withName(req, "orders")
		Struct{}.RotateGatewayJWTConsumer(recorder, req)
		if recorder.Code != http.StatusOK || stub.rotateCalls != 1 || stub.lastNamespace != "team-a" || stub.lastName != "orders" {
			t.Fatalf("rotate forwarding: status=%d calls=%d namespace=%q name=%q body=%s", recorder.Code, stub.rotateCalls, stub.lastNamespace, stub.lastName, recorder.Body.String())
		}
	})

	t.Run("delete", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := withTenant(httptest.NewRequest(http.MethodDelete, "/orders", nil))
		req = withName(req, "orders")
		Struct{}.DeleteGatewayJWTConsumer(recorder, req)
		if recorder.Code != http.StatusOK || stub.deleteCalls != 1 || stub.lastNamespace != "team-a" || stub.lastName != "orders" {
			t.Fatalf("delete forwarding: status=%d calls=%d namespace=%q name=%q", recorder.Code, stub.deleteCalls, stub.lastNamespace, stub.lastName)
		}
	})
}
