package apigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	"github.com/go-chi/chi"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type tcpRouteTestManager struct {
	db.Manager
	tenantServiceDao dbdao.TenantServiceDao
	tcpRuleDao       dbdao.TCPRuleDao
}

func TestCreateHTTPAPIRouteAddsCanonicalIdentityLabels(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodPost,
		"/?appID=region-app-id&service_id=service-id&service_alias=service-alias&port=8080",
		nil,
	)
	tenant := &dbmodel.Tenants{
		UUID:      "tenant-id",
		Name:      "tenant-name",
		Namespace: "tenant-namespace",
	}

	labels := httpAPIRouteLabels(tenant, req, "service-alias")
	want := map[string]string{
		"creator":        "Rainbond",
		"tenant_id":      "tenant-id",
		"tenant_name":    "tenant-name",
		"app_id":         "region-app-id",
		"service_id":     "service-id",
		"service_alias":  "service-alias",
		"port":           "8080",
		"component_sort": "service-alias",
		"service-alias":  "service_alias",
	}
	for key, expected := range want {
		if got := labels[key]; got != expected {
			t.Errorf("label %s = %q; want %q", key, got, expected)
		}
	}
}

func (m tcpRouteTestManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.tenantServiceDao
}

func (m tcpRouteTestManager) TCPRuleDao() dbdao.TCPRuleDao {
	return m.tcpRuleDao
}

type tcpRouteTenantServiceDao struct {
	dbdao.TenantServiceDao
	servicesByID map[string]*dbmodel.TenantServices
}

func (d *tcpRouteTenantServiceDao) GetServiceByTenantIDAndServiceAlias(tenantID, serviceName string) (*dbmodel.TenantServices, error) {
	return nil, gorm.ErrRecordNotFound
}

func (d *tcpRouteTenantServiceDao) GetServiceByID(serviceID string) (*dbmodel.TenantServices, error) {
	service, ok := d.servicesByID[serviceID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return service, nil
}

type tcpRouteRuleDao struct {
	dbdao.TCPRuleDao
	added             *dbmodel.TCPRule
	replaced          *dbmodel.TCPRule
	rules             []*dbmodel.TCPRule
	deletedIDs        []uint
	beforeDeleteByIDs func()
	afterListByIPPort func()
}

func (d *tcpRouteRuleDao) AddModel(m dbmodel.Interface) error {
	d.added = m.(*dbmodel.TCPRule)
	return nil
}

func (d *tcpRouteRuleDao) GetUsedPortsByIP(string) ([]*dbmodel.TCPRule, error) {
	return d.rules, nil
}

func (d *tcpRouteRuleDao) ReplaceByIPAndPort(rule *dbmodel.TCPRule) error {
	canonical := *rule
	d.replaced = &canonical
	remaining := make([]*dbmodel.TCPRule, 0, len(d.rules)+1)
	for _, existing := range d.rules {
		if existing.IP == rule.IP && existing.Port == rule.Port {
			continue
		}
		if rule.UUID != "" && existing.UUID == rule.UUID {
			continue
		}
		remaining = append(remaining, existing)
	}
	remaining = append(remaining, &canonical)
	d.rules = remaining
	return nil
}

func (d *tcpRouteRuleDao) ListByIPAndPort(ip string, port int) ([]*dbmodel.TCPRule, error) {
	var rules []*dbmodel.TCPRule
	for _, rule := range d.rules {
		if rule.IP == ip && rule.Port == port {
			rules = append(rules, rule)
		}
	}
	if d.afterListByIPPort != nil {
		d.afterListByIPPort()
	}
	return rules, nil
}

func (d *tcpRouteRuleDao) DeleteByRecordIDs(ids []uint) error {
	d.deletedIDs = append(d.deletedIDs, ids...)
	if d.beforeDeleteByIDs != nil {
		d.beforeDeleteByIDs()
	}
	deleted := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		deleted[id] = struct{}{}
	}
	remaining := make([]*dbmodel.TCPRule, 0, len(d.rules))
	for _, rule := range d.rules {
		if _, ok := deleted[rule.ID]; !ok {
			remaining = append(remaining, rule)
		}
	}
	d.rules = remaining
	return nil
}

func newTCPRouteTestClientset(t *testing.T, services map[string]*corev1.Service) (*kubernetes.Clientset, func()) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	codecs := serializer.NewCodecFactory(scheme)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/namespaces/default/services" {
			serviceList := corev1.ServiceList{}
			for _, service := range services {
				serviceList.Items = append(serviceList.Items, *service)
			}
			if err := json.NewEncoder(w).Encode(&serviceList); err != nil {
				t.Fatalf("encode service list: %v", err)
			}
			return
		}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/namespaces/default/services/") {
			name := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/default/services/")
			if service, ok := services[name]; ok {
				if err := json.NewEncoder(w).Encode(service); err != nil {
					t.Fatalf("encode service: %v", err)
				}
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(v1.Status{
				TypeMeta: v1.TypeMeta{
					Kind:       "Status",
					APIVersion: "v1",
				},
				Status: v1.StatusFailure,
				Reason: v1.StatusReasonNotFound,
				Code:   http.StatusNotFound,
			})
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/namespaces/default/services" {
			var service corev1.Service
			if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
				t.Fatalf("decode service: %v", err)
			}
			for _, existing := range services {
				for _, existingPort := range existing.Spec.Ports {
					for _, requestedPort := range service.Spec.Ports {
						if requestedPort.NodePort == 0 || requestedPort.NodePort != existingPort.NodePort {
							continue
						}
						w.WriteHeader(http.StatusUnprocessableEntity)
						_ = json.NewEncoder(w).Encode(v1.Status{
							TypeMeta: v1.TypeMeta{Kind: "Status", APIVersion: "v1"},
							Status:   v1.StatusFailure,
							Message: fmt.Sprintf(
								"Service %q is invalid: spec.ports[0].nodePort: Invalid value: %d: provided port is already allocated",
								service.Name,
								requestedPort.NodePort,
							),
							Reason: v1.StatusReasonInvalid,
							Code:   http.StatusUnprocessableEntity,
						})
						return
					}
				}
			}
			service.Namespace = "default"
			services[service.Name] = &service
			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(&service); err != nil {
				t.Fatalf("encode created service: %v", err)
			}
			return
		}
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/v1/namespaces/default/services/") {
			name := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/default/services/")
			existing, ok := services[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(v1.Status{
					TypeMeta: v1.TypeMeta{Kind: "Status", APIVersion: "v1"},
					Status:   v1.StatusFailure,
					Reason:   v1.StatusReasonNotFound,
					Code:     http.StatusNotFound,
				})
				return
			}
			var service corev1.Service
			if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
				t.Fatalf("decode updated service: %v", err)
			}
			service.Namespace = "default"
			if service.UID == "" {
				service.UID = existing.UID
			}
			services[name] = &service
			if err := json.NewEncoder(w).Encode(&service); err != nil {
				t.Fatalf("encode updated service: %v", err)
			}
			return
		}
		if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/namespaces/default/services/") {
			name := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/default/services/")
			existing, ok := services[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(v1.Status{
					TypeMeta: v1.TypeMeta{
						Kind:       "Status",
						APIVersion: "v1",
					},
					Status: v1.StatusFailure,
					Reason: v1.StatusReasonNotFound,
					Code:   http.StatusNotFound,
				})
				return
			}
			var options v1.DeleteOptions
			_ = json.NewDecoder(r.Body).Decode(&options)
			if options.Preconditions != nil && options.Preconditions.UID != nil && *options.Preconditions.UID != existing.UID {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(v1.Status{
					TypeMeta: v1.TypeMeta{Kind: "Status", APIVersion: "v1"},
					Status:   v1.StatusFailure,
					Message:  fmt.Sprintf("Operation cannot be fulfilled on services %q: UID precondition failed", name),
					Reason:   v1.StatusReasonConflict,
					Code:     http.StatusConflict,
				})
				return
			}
			delete(services, name)
			_ = json.NewEncoder(w).Encode(v1.Status{
				TypeMeta: v1.TypeMeta{
					Kind:       "Status",
					APIVersion: "v1",
				},
				Status: v1.StatusSuccess,
				Code:   http.StatusOK,
			})
			return
		}
		t.Fatalf("unexpected kubernetes request: %s %s", r.Method, r.URL.Path)
	}))
	config := &rest.Config{
		Host: server.URL,
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &schema.GroupVersion{Version: "v1"},
			NegotiatedSerializer: codecs.WithoutConversion(),
		},
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("create clientset: %v", err)
	}
	return clientset, server.Close
}

func TestParseCertManagerDomains(t *testing.T) {
	domains := parseCertManagerDomains("foo.example.com, bar.example.com ,,baz.example.com")
	if len(domains) != 3 {
		t.Fatalf("expected 3 domains, got %d", len(domains))
	}
	if domains[0] != "foo.example.com" || domains[1] != "bar.example.com" || domains[2] != "baz.example.com" {
		t.Fatalf("unexpected domains: %#v", domains)
	}
}

func TestHasMatchingCertManagerDomain(t *testing.T) {
	tests := []struct {
		name         string
		certDomains  []string
		routeDomains []string
		want         bool
	}{
		{
			name:         "exact match",
			certDomains:  []string{"foo.example.com"},
			routeDomains: []string{"foo.example.com"},
			want:         true,
		},
		{
			name:         "wildcard match",
			certDomains:  []string{"*.example.com"},
			routeDomains: []string{"foo.example.com"},
			want:         false,
		},
		{
			name:         "no overlap",
			certDomains:  []string{"foo.example.com"},
			routeDomains: []string{"bar.example.com"},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasMatchingCertManagerDomain(tt.certDomains, tt.routeDomains)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestRouteMatchesCertManagerDomains(t *testing.T) {
	route := &v2.ApisixRoute{
		Spec: v2.ApisixRouteSpec{
			HTTP: []v2.ApisixRouteHTTP{
				{
					Match: v2.ApisixRouteHTTPMatch{
						Hosts: []string{"foo.example.com", "bar.example.com"},
					},
				},
			},
		},
	}

	if !routeMatchesCertManagerDomains(route, []string{"bar.example.com"}) {
		t.Fatal("expected route to match certificate domains")
	}

	if routeMatchesCertManagerDomains(route, []string{"baz.example.com"}) {
		t.Fatal("expected route not to match unrelated certificate domains")
	}
}

func TestCreateTCPRouteUsesRainbondServiceAliasFromBackendServiceLabels(t *testing.T) {
	const (
		namespace    = "default"
		tenantID     = "tenant-id"
		appID        = "app-id"
		serviceID    = "db66afd0892c326ff557df7880ac572d"
		serviceAlias = "grac572d"
		serviceName  = "demo-2048"
		nodePort     = int32(30000)
	)

	services := map[string]*corev1.Service{
		serviceName: {
			ObjectMeta: v1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				Labels: map[string]string{
					"app_id":        appID,
					"service_id":    serviceID,
					"service_alias": serviceAlias,
					"rainbond_app":  serviceName,
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name:       "http-8080",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				}},
				Selector: map[string]string{"name": serviceAlias},
			},
		},
	}
	clientset, closeServer := newTCPRouteTestClientset(t, services)
	defer closeServer()
	k8s.New().Clientset = clientset

	ruleDao := &tcpRouteRuleDao{}
	db.SetTestManager(tcpRouteTestManager{
		tenantServiceDao: &tcpRouteTenantServiceDao{servicesByID: map[string]*dbmodel.TenantServices{
			serviceID: {
				ServiceID:        serviceID,
				ServiceAlias:     serviceAlias,
				TenantID:         tenantID,
				ExtendMethod:     "",
				K8sComponentName: serviceAlias,
			},
		}},
		tcpRuleDao: ruleDao,
	})
	defer db.SetTestManager(nil)

	streamRoute := v2.ApisixRouteStream{
		Name:     "tcp",
		Protocol: "tcp",
		Match: v2.ApisixRouteStreamMatch{
			IngressPort: nodePort,
		},
		Backend: v2.ApisixRouteStreamBackend{
			ServiceName: serviceName,
			ServicePort: intstr.FromInt(8080),
		},
	}
	body, err := json.Marshal(streamRoute)
	if err != nil {
		t.Fatalf("marshal route: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/?appID="+appID, bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), ctxutil.ContextKey("tenant"), &dbmodel.Tenants{
		UUID:      tenantID,
		Namespace: namespace,
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	Struct{}.CreateTCPRoute(rr, req)

	created, err := k8s.Default().Clientset.CoreV1().Services(namespace).Get(context.Background(), serviceName+"-30000", v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			t.Fatalf("expected NodePort service to be created")
		}
		t.Fatalf("get created service: %v", err)
	}
	if got := created.Spec.Selector["service_alias"]; got != serviceAlias {
		t.Fatalf("expected selector service_alias %q, got %q", serviceAlias, got)
	}
	if got := created.Labels["service_alias"]; got != serviceAlias {
		t.Fatalf("expected label service_alias %q, got %q", serviceAlias, got)
	}
	if got := created.Labels["service_id"]; got != serviceID {
		t.Fatalf("expected label service_id %q, got %q", serviceID, got)
	}
	if ruleDao.replaced == nil {
		t.Fatal("expected TCP rule to be reconciled")
	}
	if got := ruleDao.replaced.ServiceID; got != serviceID {
		t.Fatalf("expected TCP rule service_id %q, got %q", serviceID, got)
	}
}

// capability_id: rainbond.gateway.reconcile-stale-tcp-nodeport-rules
func TestCreateTCPRouteReconcilesStaleDatabaseRules(t *testing.T) {
	const (
		namespace     = "default"
		tenantID      = "tenant-id"
		serviceID     = "service-b"
		serviceAlias  = "service-b-alias"
		serviceName   = "service-b-name"
		requestedPort = int32(30000)
	)

	services := map[string]*corev1.Service{}
	clientset, closeServer := newTCPRouteTestClientset(t, services)
	defer closeServer()
	k8s.New().Clientset = clientset

	ruleDao := &tcpRouteRuleDao{rules: []*dbmodel.TCPRule{
		{
			Model:     dbmodel.Model{ID: 1},
			UUID:      "service-a",
			ServiceID: "service-a",
			IP:        "0.0.0.0",
			Port:      int(requestedPort),
		},
		{
			Model: dbmodel.Model{ID: 2},
			IP:    "0.0.0.0",
			Port:  int(requestedPort),
		},
	}}
	db.SetTestManager(tcpRouteTestManager{
		tenantServiceDao: &tcpRouteTenantServiceDao{servicesByID: map[string]*dbmodel.TenantServices{
			serviceID: {
				ServiceID:    serviceID,
				ServiceAlias: serviceAlias,
				TenantID:     tenantID,
			},
		}},
		tcpRuleDao: ruleDao,
	})
	defer db.SetTestManager(nil)

	rr := createTCPRouteForTest(t, namespace, tenantID, serviceID, serviceName, requestedPort)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if ruleDao.replaced == nil {
		t.Fatal("expected stale TCP rules to be reconciled")
	}
	if ruleDao.replaced.ServiceID != serviceID || ruleDao.replaced.Port != int(requestedPort) {
		t.Fatalf("unexpected canonical TCP rule: %#v", ruleDao.replaced)
	}
	if len(ruleDao.rules) != 1 || ruleDao.rules[0].ServiceID != serviceID {
		t.Fatalf("expected one canonical TCP rule, got %#v", ruleDao.rules)
	}
	if _, ok := services[serviceName+"-30000"]; !ok {
		t.Fatal("expected Kubernetes NodePort service to be created")
	}
}

// capability_id: rainbond.gateway.reject-duplicate-tcp-nodeport
func TestCreateTCPRouteRejectsExplicitPortOwnedByAnotherService(t *testing.T) {
	const (
		namespace     = "default"
		tenantID      = "tenant-id"
		serviceID     = "service-b"
		serviceAlias  = "service-b-alias"
		serviceName   = "service-b-name"
		requestedPort = int32(30000)
	)

	services := map[string]*corev1.Service{
		"service-a-30000": {
			ObjectMeta: v1.ObjectMeta{
				Name:      "service-a-30000",
				Namespace: namespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name:     "service-a-30000",
					Protocol: corev1.ProtocolTCP,
					Port:     8080,
					NodePort: requestedPort,
				}},
				Type: corev1.ServiceTypeNodePort,
			},
		},
	}
	clientset, closeServer := newTCPRouteTestClientset(t, services)
	defer closeServer()
	k8s.New().Clientset = clientset

	ruleDao := &tcpRouteRuleDao{}
	db.SetTestManager(tcpRouteTestManager{
		tenantServiceDao: &tcpRouteTenantServiceDao{servicesByID: map[string]*dbmodel.TenantServices{
			serviceID: {
				ServiceID:    serviceID,
				ServiceAlias: serviceAlias,
				TenantID:     tenantID,
			},
		}},
		tcpRuleDao: ruleDao,
	})
	defer db.SetTestManager(nil)

	streamRoute := v2.ApisixRouteStream{
		Name:     "tcp",
		Protocol: "tcp",
		Match: v2.ApisixRouteStreamMatch{
			IngressPort: requestedPort,
		},
		Backend: v2.ApisixRouteStreamBackend{
			ServiceName: serviceName,
			ServicePort: intstr.FromInt(9090),
		},
	}
	body, err := json.Marshal(streamRoute)
	if err != nil {
		t.Fatalf("marshal route: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/?service_id="+serviceID, bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), ctxutil.ContextKey("tenant"), &dbmodel.Tenants{
		UUID:      tenantID,
		Namespace: namespace,
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	Struct{}.CreateTCPRoute(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rr.Code, rr.Body.String())
	}
	var response struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if response.Code != 50005 {
		t.Fatalf("expected error code 50005, got %d: %s", response.Code, rr.Body.String())
	}
	if ruleDao.replaced != nil {
		t.Fatalf("expected conflicting TCP rule not to be reconciled, got %#v", ruleDao.replaced)
	}
	if _, ok := services[serviceName+"-30000"]; ok {
		t.Fatal("expected conflicting NodePort service not to be created")
	}
}

// capability_id: rainbond.gateway.protect-tcp-route-service-ownership
func TestCreateTCPRouteRejectsExistingServiceWithoutMatchingOwner(t *testing.T) {
	const (
		namespace     = "default"
		tenantID      = "tenant-id"
		serviceID     = "service-a"
		serviceAlias  = "service-a-alias"
		serviceName   = "service-a-name"
		requestedPort = int32(30000)
		routeName     = "service-a-name-30000"
	)

	tests := []struct {
		name              string
		existingServiceID string
	}{
		{name: "missing owner label"},
		{name: "different owner label", existingServiceID: "service-b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := map[string]string{"tcp": "true", "outer": "true"}
			if tt.existingServiceID != "" {
				labels["service_id"] = tt.existingServiceID
			}
			services := map[string]*corev1.Service{
				routeName: {
					ObjectMeta: v1.ObjectMeta{
						Name:      routeName,
						Namespace: namespace,
						Labels:    labels,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{
							Name:     routeName,
							Protocol: corev1.ProtocolTCP,
							Port:     8080,
							NodePort: requestedPort,
						}},
						Type: corev1.ServiceTypeNodePort,
					},
				},
			}
			clientset, closeServer := newTCPRouteTestClientset(t, services)
			defer closeServer()
			k8s.New().Clientset = clientset

			ruleDao := &tcpRouteRuleDao{}
			db.SetTestManager(tcpRouteTestManager{
				tenantServiceDao: &tcpRouteTenantServiceDao{servicesByID: map[string]*dbmodel.TenantServices{
					serviceID: {
						ServiceID:    serviceID,
						ServiceAlias: serviceAlias,
						TenantID:     tenantID,
					},
				}},
				tcpRuleDao: ruleDao,
			})
			defer db.SetTestManager(nil)

			rr := createTCPRouteForTest(t, namespace, tenantID, serviceID, serviceName, requestedPort)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", rr.Code, rr.Body.String())
			}
			var response struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode ownership response: %v", err)
			}
			if response.Code != 50005 {
				t.Fatalf("expected error code 50005, got %d: %s", response.Code, rr.Body.String())
			}
			if ruleDao.replaced != nil {
				t.Fatalf("expected database rule not to be reconciled, got %#v", ruleDao.replaced)
			}
			if got := services[routeName].Spec.Ports[0].Port; got != 8080 {
				t.Fatalf("expected existing Service not to be updated, got port %d", got)
			}
		})
	}
}

func createTCPRouteForTest(t *testing.T, namespace, tenantID, serviceID, serviceName string, nodePort int32) *httptest.ResponseRecorder {
	t.Helper()
	streamRoute := v2.ApisixRouteStream{
		Name:     "tcp",
		Protocol: "tcp",
		Match: v2.ApisixRouteStreamMatch{
			IngressPort: nodePort,
		},
		Backend: v2.ApisixRouteStreamBackend{
			ServiceName: serviceName,
			ServicePort: intstr.FromInt(9090),
		},
	}
	body, err := json.Marshal(streamRoute)
	if err != nil {
		t.Fatalf("marshal route: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/?service_id="+serviceID, bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), ctxutil.ContextKey("tenant"), &dbmodel.Tenants{
		UUID:      tenantID,
		Namespace: namespace,
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	Struct{}.CreateTCPRoute(rr, req)
	return rr
}

// capability_id: rainbond.api-gateway.vm-nodeport-service-uses-local-external-traffic-policy
func TestCreateTCPRouteSetsExternalTrafficPolicyLocalForVMService(t *testing.T) {
	const (
		namespace    = "default"
		tenantID     = "tenant-id"
		appID        = "app-id"
		serviceID    = "7de1e7b94ccf418eac0cc0de61447979"
		serviceAlias = "gr447979"
		serviceName  = "gr447979"
		nodePort     = int32(30003)
	)

	services := map[string]*corev1.Service{
		serviceName: {
			ObjectMeta: v1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				Labels: map[string]string{
					"app_id":        appID,
					"service_id":    serviceID,
					"service_alias": serviceAlias,
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name:       "rdp-3389",
					Protocol:   corev1.ProtocolTCP,
					Port:       3389,
					TargetPort: intstr.FromInt(3389),
				}},
				Selector: map[string]string{"service_alias": serviceAlias},
			},
		},
	}
	clientset, closeServer := newTCPRouteTestClientset(t, services)
	defer closeServer()
	k8s.New().Clientset = clientset

	db.SetTestManager(tcpRouteTestManager{
		tenantServiceDao: &tcpRouteTenantServiceDao{servicesByID: map[string]*dbmodel.TenantServices{
			serviceID: {
				ServiceID:        serviceID,
				ServiceAlias:     serviceAlias,
				TenantID:         tenantID,
				ExtendMethod:     string(dbmodel.ServiceTypeVM),
				K8sComponentName: serviceAlias,
			},
		}},
		tcpRuleDao: &tcpRouteRuleDao{},
	})
	defer db.SetTestManager(nil)

	streamRoute := v2.ApisixRouteStream{
		Name:     "tcp",
		Protocol: "tcp",
		Match: v2.ApisixRouteStreamMatch{
			IngressPort: nodePort,
		},
		Backend: v2.ApisixRouteStreamBackend{
			ServiceName: serviceName,
			ServicePort: intstr.FromInt(3389),
		},
	}
	body, err := json.Marshal(streamRoute)
	if err != nil {
		t.Fatalf("marshal route: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/?appID="+appID+"&service_id="+serviceID, bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), ctxutil.ContextKey("tenant"), &dbmodel.Tenants{
		UUID:      tenantID,
		Namespace: namespace,
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	Struct{}.CreateTCPRoute(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	created, err := k8s.Default().Clientset.CoreV1().Services(namespace).Get(context.Background(), serviceName+"-30003", v1.GetOptions{})
	if err != nil {
		t.Fatalf("get created service: %v", err)
	}
	if created.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyLocal {
		t.Fatalf("expected VM NodePort service externalTrafficPolicy %q, got %q", corev1.ServiceExternalTrafficPolicyLocal, created.Spec.ExternalTrafficPolicy)
	}
}

func TestGetTCPRouteIncludesServiceMetadata(t *testing.T) {
	const (
		namespace    = "default"
		appID        = "4d0f77e042f94ae2a77552fe7b595faf"
		serviceID    = "7de1e7b94ccf418eac0cc0de61447979"
		serviceAlias = "gr447979"
		serviceName  = "gr447979-30003"
		nodePort     = int32(30003)
	)

	services := map[string]*corev1.Service{
		serviceName: {
			ObjectMeta: v1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				Labels: map[string]string{
					"app_id":        appID,
					"service_id":    serviceID,
					"service_alias": serviceAlias,
					"port":          "80",
					"tcp":           "true",
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name:       serviceName,
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(80),
					NodePort:   nodePort,
				}},
				Selector: map[string]string{"service_alias": serviceAlias},
				Type:     corev1.ServiceTypeNodePort,
			},
		},
	}
	clientset, closeServer := newTCPRouteTestClientset(t, services)
	defer closeServer()
	k8s.New().Clientset = clientset

	req := httptest.NewRequest(http.MethodGet, "/?appID="+appID, nil)
	ctx := context.WithValue(req.Context(), ctxutil.ContextKey("tenant"), &dbmodel.Tenants{
		Namespace: namespace,
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	Struct{}.GetTCPRoute(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		List []struct {
			Name          string `json:"name"`
			Port          int32  `json:"port"`
			NodePort      int32  `json:"nodePort"`
			ServiceName   string `json:"service_name"`
			ServiceAlias  string `json:"service_alias"`
			ServiceID     string `json:"service_id"`
			AppID         string `json:"app_id"`
			ContainerPort int32  `json:"container_port"`
		} `json:"list"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.List) != 1 {
		t.Fatalf("expected one route, got %#v", resp.List)
	}
	got := resp.List[0]
	if got.Name != serviceName || got.Port != 80 || got.NodePort != nodePort {
		t.Fatalf("unexpected service port fields: %#v", got)
	}
	if got.ServiceName != serviceName {
		t.Fatalf("expected service_name %q, got %q", serviceName, got.ServiceName)
	}
	if got.ServiceAlias != serviceAlias {
		t.Fatalf("expected service_alias %q, got %q", serviceAlias, got.ServiceAlias)
	}
	if got.ServiceID != serviceID {
		t.Fatalf("expected service_id %q, got %q", serviceID, got.ServiceID)
	}
	if got.AppID != appID {
		t.Fatalf("expected app_id %q, got %q", appID, got.AppID)
	}
	if got.ContainerPort != 80 {
		t.Fatalf("expected container_port 80, got %d", got.ContainerPort)
	}
}

func TestDeleteTCPRouteFallsBackToNodePortWhenNameDiffers(t *testing.T) {
	const (
		namespace         = "default"
		serviceAlias      = "gr447979"
		actualServiceName = "gr447979-30003"
		requestedName     = "custom-k8s-service-30003"
		nodePort          = int32(30003)
	)

	services := map[string]*corev1.Service{
		actualServiceName: {
			ObjectMeta: v1.ObjectMeta{
				Name:      actualServiceName,
				Namespace: namespace,
				Labels: map[string]string{
					"service_alias": serviceAlias,
					"outer":         "true",
					"tcp":           "true",
					"port":          "6379",
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name:       actualServiceName,
					Protocol:   corev1.ProtocolTCP,
					Port:       6379,
					TargetPort: intstr.FromInt(6379),
					NodePort:   nodePort,
				}},
				Type: corev1.ServiceTypeNodePort,
			},
		},
	}
	clientset, closeServer := newTCPRouteTestClientset(t, services)
	defer closeServer()
	k8s.New().Clientset = clientset
	ruleDao := &tcpRouteRuleDao{}
	db.SetTestManager(tcpRouteTestManager{tcpRuleDao: ruleDao})
	defer db.SetTestManager(nil)

	req := httptest.NewRequest(http.MethodDelete, "/"+requestedName, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("name", requestedName)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	ctx = context.WithValue(ctx, ctxutil.ContextKey("tenant"), &dbmodel.Tenants{
		Namespace: namespace,
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	Struct{}.DeleteTCPRoute(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	_, err := k8s.Default().Clientset.CoreV1().Services(namespace).Get(context.Background(), actualServiceName, v1.GetOptions{})
	if !errors.IsNotFound(err) {
		t.Fatalf("expected actual TCP route service to be deleted, got err %v", err)
	}
}

// capability_id: rainbond.gateway.release-captured-tcp-nodeport-rules
func TestDeleteTCPRouteReleasesOnlyCapturedRuleIDs(t *testing.T) {
	const (
		namespace   = "default"
		serviceID   = "service-a"
		serviceName = "service-a-30003"
		nodePort    = int32(30003)
	)

	services := map[string]*corev1.Service{
		serviceName: {
			ObjectMeta: v1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				Labels: map[string]string{
					"service_id": serviceID,
					"outer":      "true",
					"tcp":        "true",
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name:     serviceName,
					Protocol: corev1.ProtocolTCP,
					Port:     8080,
					NodePort: nodePort,
				}},
				Type: corev1.ServiceTypeNodePort,
			},
		},
	}
	clientset, closeServer := newTCPRouteTestClientset(t, services)
	defer closeServer()
	k8s.New().Clientset = clientset

	ruleDao := &tcpRouteRuleDao{rules: []*dbmodel.TCPRule{
		{Model: dbmodel.Model{ID: 10}, UUID: serviceID, ServiceID: serviceID, IP: "0.0.0.0", Port: int(nodePort)},
		{Model: dbmodel.Model{ID: 11}, IP: "0.0.0.0", Port: int(nodePort)},
		{Model: dbmodel.Model{ID: 12}, UUID: serviceID, ServiceID: serviceID, IP: "0.0.0.0", Port: 30004},
	}}
	ruleDao.beforeDeleteByIDs = func() {
		ruleDao.rules = append(ruleDao.rules, &dbmodel.TCPRule{
			Model:     dbmodel.Model{ID: 13},
			UUID:      "service-b",
			ServiceID: "service-b",
			IP:        "0.0.0.0",
			Port:      int(nodePort),
		})
	}
	db.SetTestManager(tcpRouteTestManager{tcpRuleDao: ruleDao})
	defer db.SetTestManager(nil)

	rr := deleteTCPRouteForTest(t, namespace, serviceName, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(ruleDao.deletedIDs) != 2 || ruleDao.deletedIDs[0] != 10 || ruleDao.deletedIDs[1] != 11 {
		t.Fatalf("expected captured IDs [10 11] to be deleted, got %v", ruleDao.deletedIDs)
	}
	if len(ruleDao.rules) != 2 || ruleDao.rules[0].ID != 12 || ruleDao.rules[1].ID != 13 {
		t.Fatalf("expected non-captured and future rules to remain, got %#v", ruleDao.rules)
	}
}

// capability_id: rainbond.gateway.prevent-tcp-route-delete-recreation-race
func TestDeleteTCPRouteDoesNotDeleteRecreatedServiceWithStaleUID(t *testing.T) {
	const (
		namespace   = "default"
		serviceID   = "service-a"
		serviceName = "service-a-30003"
		nodePort    = int32(30003)
	)

	services := map[string]*corev1.Service{
		serviceName: {
			ObjectMeta: v1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				UID:       "old-uid",
				Labels: map[string]string{
					"service_id": serviceID,
					"outer":      "true",
					"tcp":        "true",
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name:     serviceName,
					Protocol: corev1.ProtocolTCP,
					Port:     8080,
					NodePort: nodePort,
				}},
				Type: corev1.ServiceTypeNodePort,
			},
		},
	}
	clientset, closeServer := newTCPRouteTestClientset(t, services)
	defer closeServer()
	k8s.New().Clientset = clientset

	ruleDao := &tcpRouteRuleDao{rules: []*dbmodel.TCPRule{
		{Model: dbmodel.Model{ID: 40}, UUID: serviceID, ServiceID: serviceID, IP: "0.0.0.0", Port: int(nodePort)},
	}}
	ruleDao.afterListByIPPort = func() {
		ruleDao.afterListByIPPort = nil
		services[serviceName] = &corev1.Service{
			ObjectMeta: v1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				UID:       "new-uid",
				Labels: map[string]string{
					"service_id": "service-b",
					"outer":      "true",
					"tcp":        "true",
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name:     serviceName,
					Protocol: corev1.ProtocolTCP,
					Port:     9090,
					NodePort: nodePort,
				}},
				Type: corev1.ServiceTypeNodePort,
			},
		}
	}
	db.SetTestManager(tcpRouteTestManager{tcpRuleDao: ruleDao})
	defer db.SetTestManager(nil)

	rr := deleteTCPRouteForTest(t, namespace, serviceName, serviceID)
	if rr.Code == http.StatusOK {
		t.Fatalf("expected stale UID delete to fail, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := services[serviceName]; got == nil || got.UID != "new-uid" {
		t.Fatalf("expected recreated Service to remain, got %#v", got)
	}
	if len(ruleDao.deletedIDs) != 0 || len(ruleDao.rules) != 1 || ruleDao.rules[0].ID != 40 {
		t.Fatalf("expected captured rules to remain after failed UID precondition, deleted=%v rules=%#v", ruleDao.deletedIDs, ruleDao.rules)
	}
}

// capability_id: rainbond.gateway.protect-unlabeled-tcp-route-service
func TestDeleteTCPRouteWithServiceIDDoesNotDeleteUnlabeledService(t *testing.T) {
	const (
		namespace   = "default"
		serviceID   = "service-a"
		serviceName = "service-a-30003"
		nodePort    = int32(30003)
	)

	services := map[string]*corev1.Service{
		serviceName: {
			ObjectMeta: v1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				UID:       "unknown-owner-uid",
				Labels: map[string]string{
					"outer": "true",
					"tcp":   "true",
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name:     serviceName,
					Protocol: corev1.ProtocolTCP,
					Port:     8080,
					NodePort: nodePort,
				}},
				Type: corev1.ServiceTypeNodePort,
			},
		},
	}
	clientset, closeServer := newTCPRouteTestClientset(t, services)
	defer closeServer()
	k8s.New().Clientset = clientset

	ruleDao := &tcpRouteRuleDao{rules: []*dbmodel.TCPRule{
		{Model: dbmodel.Model{ID: 50}, UUID: serviceID, ServiceID: serviceID, IP: "0.0.0.0", Port: int(nodePort)},
		{Model: dbmodel.Model{ID: 51}, IP: "0.0.0.0", Port: int(nodePort)},
		{Model: dbmodel.Model{ID: 52}, UUID: "service-b", ServiceID: "service-b", IP: "0.0.0.0", Port: int(nodePort)},
	}}
	db.SetTestManager(tcpRouteTestManager{tcpRuleDao: ruleDao})
	defer db.SetTestManager(nil)

	rr := deleteTCPRouteForTest(t, namespace, serviceName, serviceID)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected unknown owner to be treated as already deleted, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := services[serviceName]; got == nil || got.UID != "unknown-owner-uid" {
		t.Fatalf("expected unlabeled Service to remain, got %#v", got)
	}
	if len(ruleDao.deletedIDs) != 1 || ruleDao.deletedIDs[0] != 50 {
		t.Fatalf("expected only requested owner's old rule to be deleted, got %v", ruleDao.deletedIDs)
	}
	if len(ruleDao.rules) != 2 || ruleDao.rules[0].ID != 51 || ruleDao.rules[1].ID != 52 {
		t.Fatalf("expected unknown-owner rules to remain, got %#v", ruleDao.rules)
	}
}

// capability_id: rainbond.gateway.release-absent-tcp-nodeport-owner
func TestDeleteTCPRouteAlreadyAbsentReleasesOnlyRequestedOwner(t *testing.T) {
	const (
		namespace   = "default"
		serviceID   = "service-a"
		serviceName = "service-a-30003"
		nodePort    = int32(30003)
	)

	services := map[string]*corev1.Service{}
	clientset, closeServer := newTCPRouteTestClientset(t, services)
	defer closeServer()
	k8s.New().Clientset = clientset

	ruleDao := &tcpRouteRuleDao{rules: []*dbmodel.TCPRule{
		{Model: dbmodel.Model{ID: 20}, UUID: serviceID, ServiceID: serviceID, IP: "0.0.0.0", Port: int(nodePort)},
		{Model: dbmodel.Model{ID: 21}, IP: "0.0.0.0", Port: int(nodePort)},
		{Model: dbmodel.Model{ID: 22}, UUID: "service-b", ServiceID: "service-b", IP: "0.0.0.0", Port: int(nodePort)},
	}}
	db.SetTestManager(tcpRouteTestManager{tcpRuleDao: ruleDao})
	defer db.SetTestManager(nil)

	rr := deleteTCPRouteForTest(t, namespace, serviceName, serviceID)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(ruleDao.deletedIDs) != 1 || ruleDao.deletedIDs[0] != 20 {
		t.Fatalf("expected only requested owner's rule ID to be deleted, got %v", ruleDao.deletedIDs)
	}
	if len(ruleDao.rules) != 2 || ruleDao.rules[0].ID != 21 || ruleDao.rules[1].ID != 22 {
		t.Fatalf("expected anonymous and future-owner rules to remain, got %#v", ruleDao.rules)
	}
}

func TestDeleteTCPRouteAlreadyAbsentWithoutServiceIDKeepsRules(t *testing.T) {
	const (
		namespace   = "default"
		serviceName = "service-a-30003"
	)

	services := map[string]*corev1.Service{}
	clientset, closeServer := newTCPRouteTestClientset(t, services)
	defer closeServer()
	k8s.New().Clientset = clientset

	ruleDao := &tcpRouteRuleDao{rules: []*dbmodel.TCPRule{
		{Model: dbmodel.Model{ID: 30}, UUID: "service-b", ServiceID: "service-b", IP: "0.0.0.0", Port: 30003},
	}}
	db.SetTestManager(tcpRouteTestManager{tcpRuleDao: ruleDao})
	defer db.SetTestManager(nil)

	rr := deleteTCPRouteForTest(t, namespace, serviceName, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(ruleDao.deletedIDs) != 0 || len(ruleDao.rules) != 1 {
		t.Fatalf("expected old client request not to release an unknown owner, got deleted=%v rules=%#v", ruleDao.deletedIDs, ruleDao.rules)
	}
}

func deleteTCPRouteForTest(t *testing.T, namespace, serviceName, serviceID string) *httptest.ResponseRecorder {
	t.Helper()
	path := "/" + serviceName
	if serviceID != "" {
		path += "?service_id=" + serviceID
	}
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("name", serviceName)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	ctx = context.WithValue(ctx, ctxutil.ContextKey("tenant"), &dbmodel.Tenants{
		Namespace: namespace,
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	Struct{}.DeleteTCPRoute(rr, req)
	return rr
}

// capability_id: rainbond.gateway.validate-tcp-nodeport-route-name
func TestNodePortFromTCPRouteNameValidatesRange(t *testing.T) {
	tests := []struct {
		name      string
		routeName string
		wantPort  int32
		wantOK    bool
	}{
		{name: "valid", routeName: "service-a-30003", wantPort: 30003, wantOK: true},
		{name: "zero", routeName: "service-a-0"},
		{name: "above maximum", routeName: "service-a-65536"},
		{name: "int32 wraparound", routeName: "service-a-4294997299"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPort, gotOK := nodePortFromTCPRouteName(tt.routeName)
			if gotPort != tt.wantPort || gotOK != tt.wantOK {
				t.Fatalf("nodePortFromTCPRouteName(%q) = (%d, %v), want (%d, %v)", tt.routeName, gotPort, gotOK, tt.wantPort, tt.wantOK)
			}
		})
	}
}
