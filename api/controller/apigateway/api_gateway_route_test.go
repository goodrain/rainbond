package apigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
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
	added *dbmodel.TCPRule
}

func (d *tcpRouteRuleDao) AddModel(m dbmodel.Interface) error {
	d.added = m.(*dbmodel.TCPRule)
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
			service.Namespace = "default"
			services[service.Name] = &service
			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(&service); err != nil {
				t.Fatalf("encode created service: %v", err)
			}
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
	if ruleDao.added == nil {
		t.Fatal("expected TCP rule to be persisted")
	}
	if got := ruleDao.added.ServiceID; got != serviceID {
		t.Fatalf("expected TCP rule service_id %q, got %q", serviceID, got)
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
