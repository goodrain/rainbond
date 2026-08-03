package handler

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"reflect"
	"testing"

	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	apisixversioned "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned"
	apisixfake "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned/fake"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type gatewayRouteDeleteEventTestManager struct {
	db.Manager
	portDao  dbdao.TenantServicesPortDao
	eventDao dbdao.EventDao
}

func (m gatewayRouteDeleteEventTestManager) TenantServicesPortDao() dbdao.TenantServicesPortDao {
	return m.portDao
}

func (m gatewayRouteDeleteEventTestManager) ServiceEventDao() dbdao.EventDao {
	return m.eventDao
}

type gatewayRouteDeleteEventPortDao struct {
	dbdao.TenantServicesPortDao
	portsByName map[string]*dbmodel.TenantServicesPort
}

func (d *gatewayRouteDeleteEventPortDao) ListByK8sServiceNames(names []string) ([]*dbmodel.TenantServicesPort, error) {
	var ports []*dbmodel.TenantServicesPort
	for _, name := range names {
		if port, ok := d.portsByName[name]; ok {
			ports = append(ports, port)
		}
	}
	return ports, nil
}

type gatewayRouteDeleteEventDao struct {
	dbdao.EventDao
	events []*dbmodel.ServiceEvent
}

func (d *gatewayRouteDeleteEventDao) AddModel(arg dbmodel.Interface) error {
	d.events = append(d.events, arg.(*dbmodel.ServiceEvent))
	return nil
}

// capability_id: rainbond.gateway.allocate-lb-port
func TestSelectAvailablePort(t *testing.T) {
	// 设置环境变量
	os.Setenv("MIN_LB_PORT", "30000")
	os.Setenv("MAX_LB_PORT", "65535")

	tests := []struct {
		name     string
		used     []int
		expected int
	}{
		{
			name:     "空列表，返回最小端口",
			used:     []int{},
			expected: 30000,
		},
		{
			name:     "连续端口，返回下一个",
			used:     []int{30000, 30001, 30002},
			expected: 30003,
		},
		{
			name:     "有间隙，返回第一个空闲端口",
			used:     []int{30000, 30002, 30003},
			expected: 30001,
		},
		{
			name:     "大间隙，返回第一个空闲端口",
			used:     []int{30000, 32077},
			expected: 30001,
		},
		{
			name:     "乱序输入，返回第一个空闲端口",
			used:     []int{30002, 30000, 30003},
			expected: 30001,
		},
		{
			name:     "从中间开始有间隙",
			used:     []int{30000, 30001, 30002, 30005, 30006},
			expected: 30003,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectAvailablePort(tt.used)
			if result != tt.expected {
				t.Errorf("selectAvailablePort(%v) = %d, expected %d", tt.used, result, tt.expected)
			}
		})
	}
}

// capability_id: rainbond.gateway.reassign-conflicting-imported-tcp-port
func TestReassignConflictingTCPRulePorts(t *testing.T) {
	t.Setenv("MIN_LB_PORT", "30000")
	t.Setenv("MAX_LB_PORT", "30010")

	existing := []*dbmodel.TCPRule{
		{
			ServiceID: "source-service",
			IP:        "0.0.0.0",
			Port:      30000,
		},
	}
	incoming := []*dbmodel.TCPRule{
		{
			UUID:          "imported-rule",
			ServiceID:     "installed-service",
			ContainerPort: 8080,
			IP:            "0.0.0.0",
			Port:          30000,
		},
	}

	err := reassignConflictingTCPRulePorts(existing, incoming)

	if err != nil {
		t.Fatalf("reassignConflictingTCPRulePorts returned error: %v", err)
	}
	if incoming[0].Port != 30001 {
		t.Fatalf("incoming TCP rule port = %d, expected 30001", incoming[0].Port)
	}
}

func TestGatewayParentRefValuesHandlesEmptyParentRefs(t *testing.T) {
	gatewayName, gatewayNamespace, sectionName := gatewayParentRefValues(nil, "app-ns")

	if gatewayName != "" {
		t.Fatalf("gatewayName = %q, expected empty", gatewayName)
	}
	if gatewayNamespace != "app-ns" {
		t.Fatalf("gatewayNamespace = %q, expected default namespace", gatewayNamespace)
	}
	if sectionName != "" {
		t.Fatalf("sectionName = %q, expected empty", sectionName)
	}
}

func TestGatewayParentRefValuesReturnsFirstParentRef(t *testing.T) {
	parentNamespace := gatewayv1.Namespace("gateway-ns")
	parentSection := gatewayv1.SectionName("https")

	gatewayName, gatewayNamespace, sectionName := gatewayParentRefValues([]gatewayv1.ParentReference{
		{
			Name:        gatewayv1.ObjectName("gateway-a"),
			Namespace:   &parentNamespace,
			SectionName: &parentSection,
		},
	}, "app-ns")

	if gatewayName != "gateway-a" {
		t.Fatalf("gatewayName = %q, expected gateway-a", gatewayName)
	}
	if gatewayNamespace != "gateway-ns" {
		t.Fatalf("gatewayNamespace = %q, expected gateway-ns", gatewayNamespace)
	}
	if sectionName != "https" {
		t.Fatalf("sectionName = %q, expected https", sectionName)
	}
}

func gatewayCertificateRequestForTest(t *testing.T, name, namespace, domain string) *apimodel.GatewayCertificate {
	t.Helper()

	cert, key, err := generateSelfSignedCertificate(domain)
	if err != nil {
		t.Fatalf("generate certificate: %v", err)
	}
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal certificate key: %v", err)
	}

	return &apimodel.GatewayCertificate{
		Name:        name,
		Namespace:   namespace,
		Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})),
		PrivateKey:  string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})),
	}
}

func gatewayActionForCertificateTest(kubeClient kubernetes.Interface, apisixClient apisixversioned.Interface) *GatewayAction {
	return &GatewayAction{
		kubeClient:   kubeClient,
		apisixClient: apisixClient,
		domainConflictChecker: func(context.Context, []v2.HostType, string, string) error {
			return nil
		},
	}
}

func assertGatewayCertificateResources(t *testing.T, action *GatewayAction, req *apimodel.GatewayCertificate, domain string) {
	t.Helper()

	secret, err := action.kubeClient.CoreV1().Secrets(req.Namespace).Get(context.Background(), req.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get reconciled Secret: %v", err)
	}
	if !bytes.Equal(secret.Data[corev1.TLSCertKey], []byte(req.Certificate)) {
		t.Fatal("reconciled Secret certificate does not match request")
	}
	if !bytes.Equal(secret.Data[corev1.TLSPrivateKeyKey], []byte(req.PrivateKey)) {
		t.Fatal("reconciled Secret private key does not match request")
	}

	tls, err := action.apisixClient.ApisixV2().ApisixTlses(req.Namespace).Get(context.Background(), req.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get reconciled ApisixTls: %v", err)
	}
	if tls.Spec == nil {
		t.Fatal("reconciled ApisixTls has no spec")
	}
	if tls.Spec.IngressClassName != "apisix" {
		t.Fatalf("ingress class = %q, expected apisix", tls.Spec.IngressClassName)
	}
	if !reflect.DeepEqual(tls.Spec.Hosts, []v2.HostType{v2.HostType(domain)}) {
		t.Fatalf("hosts = %#v, expected %q", tls.Spec.Hosts, domain)
	}
	if tls.Spec.Secret.Name != req.Name || tls.Spec.Secret.Namespace != req.Namespace {
		t.Fatalf("secret reference = %s/%s, expected %s/%s", tls.Spec.Secret.Namespace, tls.Spec.Secret.Name, req.Namespace, req.Name)
	}
}

// capability_id: rainbond.gateway.certificate-resource-consistency
func TestGatewayCertificateResourceConsistency(t *testing.T) {
	const (
		name      = "gateway-cert"
		namespace = "tenant-ns"
	)

	t.Run("add creates Secret and ApisixTls", func(t *testing.T) {
		req := gatewayCertificateRequestForTest(t, name, namespace, "add.example.com")
		action := gatewayActionForCertificateTest(k8sfake.NewSimpleClientset(), apisixfake.NewSimpleClientset())

		if err := action.AddGatewayCertificate(req); err != nil {
			t.Fatalf("add gateway certificate: %v", err)
		}

		assertGatewayCertificateResources(t, action, req, "add.example.com")
	})

	t.Run("add repairs an orphan Secret", func(t *testing.T) {
		req := gatewayCertificateRequestForTest(t, name, namespace, "repaired.example.com")
		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				Namespace:       namespace,
				ResourceVersion: "7",
				Labels:          map[string]string{"keep": "label"},
			},
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte("old certificate"),
				corev1.TLSPrivateKeyKey: []byte("old private key"),
			},
			Type: corev1.SecretTypeTLS,
		}
		action := gatewayActionForCertificateTest(k8sfake.NewSimpleClientset(existing), apisixfake.NewSimpleClientset())

		if err := action.AddGatewayCertificate(req); err != nil {
			t.Fatalf("repair gateway certificate: %v", err)
		}

		assertGatewayCertificateResources(t, action, req, "repaired.example.com")
		secret, err := action.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get repaired Secret: %v", err)
		}
		if secret.Labels["keep"] != "label" {
			t.Fatalf("existing Secret metadata was not retained: %#v", secret.Labels)
		}
	})

	t.Run("new Secret is removed after ApisixTls failure", func(t *testing.T) {
		req := gatewayCertificateRequestForTest(t, name, namespace, "new-rollback.example.com")
		kubeClient := k8sfake.NewSimpleClientset()
		apisixClient := apisixfake.NewSimpleClientset()
		apisixClient.PrependReactor("create", "apisixtlses", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("apisix create failed")
		})
		action := gatewayActionForCertificateTest(kubeClient, apisixClient)

		if err := action.AddGatewayCertificate(req); err == nil {
			t.Fatal("expected add to fail")
		}

		_, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if !k8serrors.IsNotFound(err) {
			t.Fatalf("new Secret was not rolled back, get error: %v", err)
		}
	})

	t.Run("updated Secret is restored after ApisixTls failure", func(t *testing.T) {
		req := gatewayCertificateRequestForTest(t, name, namespace, "updated-rollback.example.com")
		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				Namespace:       namespace,
				ResourceVersion: "9",
				Annotations:     map[string]string{"keep": "annotation"},
			},
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte("original certificate"),
				corev1.TLSPrivateKeyKey: []byte("original private key"),
			},
			Type: corev1.SecretTypeTLS,
		}
		kubeClient := k8sfake.NewSimpleClientset(existing)
		apisixClient := apisixfake.NewSimpleClientset()
		apisixClient.PrependReactor("create", "apisixtlses", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("apisix create failed")
		})
		action := gatewayActionForCertificateTest(kubeClient, apisixClient)

		if err := action.UpdateGatewayCertificate(req); err == nil {
			t.Fatal("expected update to fail")
		}

		secret, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get restored Secret: %v", err)
		}
		if !reflect.DeepEqual(secret.Data, existing.Data) {
			t.Fatalf("restored Secret data = %#v, expected %#v", secret.Data, existing.Data)
		}
		if !reflect.DeepEqual(secret.Annotations, existing.Annotations) {
			t.Fatalf("restored Secret annotations = %#v, expected %#v", secret.Annotations, existing.Annotations)
		}
	})

	t.Run("update creates both resources when absent", func(t *testing.T) {
		req := gatewayCertificateRequestForTest(t, name, namespace, "update.example.com")
		action := gatewayActionForCertificateTest(k8sfake.NewSimpleClientset(), apisixfake.NewSimpleClientset())

		if err := action.UpdateGatewayCertificate(req); err != nil {
			t.Fatalf("update missing gateway certificate: %v", err)
		}

		assertGatewayCertificateResources(t, action, req, "update.example.com")
	})

	t.Run("update preserves existing mutual TLS client config", func(t *testing.T) {
		req := gatewayCertificateRequestForTest(t, name, namespace, "mtls.example.com")
		existingTLS := &v2.ApisixTls{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: &v2.ApisixTlsSpec{
				Client: &v2.ApisixMutualTlsClientConfig{
					CASecret: v2.ApisixSecret{Name: "client-ca", Namespace: namespace},
					Depth:    2,
				},
			},
		}
		action := gatewayActionForCertificateTest(
			k8sfake.NewSimpleClientset(),
			apisixfake.NewSimpleClientset(existingTLS),
		)

		if err := action.UpdateGatewayCertificate(req); err != nil {
			t.Fatalf("update gateway certificate: %v", err)
		}

		updatedTLS, err := action.apisixClient.ApisixV2().ApisixTlses(namespace).Get(
			context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get updated ApisixTls: %v", err)
		}
		if !reflect.DeepEqual(updatedTLS.Spec.Client, existingTLS.Spec.Client) {
			t.Fatalf("mutual TLS client config = %#v, expected %#v", updatedTLS.Spec.Client, existingTLS.Spec.Client)
		}
	})

	t.Run("delete ignores missing resources", func(t *testing.T) {
		action := gatewayActionForCertificateTest(k8sfake.NewSimpleClientset(), apisixfake.NewSimpleClientset())

		if err := action.DeleteGatewayCertificate(name, namespace); err != nil {
			t.Fatalf("delete missing gateway certificate: %v", err)
		}
	})

	t.Run("delete removes ApisixTls before Secret", func(t *testing.T) {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		tls := &v2.ApisixTls{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		kubeClient := k8sfake.NewSimpleClientset(secret)
		apisixClient := apisixfake.NewSimpleClientset(tls)
		var deletionOrder []string
		apisixClient.PrependReactor("delete", "apisixtlses", func(k8stesting.Action) (bool, runtime.Object, error) {
			deletionOrder = append(deletionOrder, "ApisixTls")
			return false, nil, nil
		})
		kubeClient.PrependReactor("delete", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
			deletionOrder = append(deletionOrder, "Secret")
			return false, nil, nil
		})
		action := gatewayActionForCertificateTest(kubeClient, apisixClient)

		if err := action.DeleteGatewayCertificate(name, namespace); err != nil {
			t.Fatalf("delete gateway certificate: %v", err)
		}
		if !reflect.DeepEqual(deletionOrder, []string{"ApisixTls", "Secret"}) {
			t.Fatalf("deletion order = %#v, expected ApisixTls then Secret", deletionOrder)
		}
	})
}

// capability_id: rainbond.gateway.http-route-delete-component-event
func TestCreateGatewayHTTPRouteDeleteEvents(t *testing.T) {
	eventDao := &gatewayRouteDeleteEventDao{}
	portDao := &gatewayRouteDeleteEventPortDao{
		portsByName: map[string]*dbmodel.TenantServicesPort{
			"svc-a": {
				TenantID:      "tenant-a",
				ServiceID:     "component-a",
				ContainerPort: 80,
			},
			"svc-b": {
				TenantID:      "tenant-b",
				ServiceID:     "component-b",
				ContainerPort: 8080,
			},
		},
	}
	db.SetTestManager(gatewayRouteDeleteEventTestManager{portDao: portDao, eventDao: eventDao})
	defer db.SetTestManager(nil)

	route := &apimodel.GatewayHTTPRouteStruct{
		Name:  "route-a",
		Hosts: []string{"example.com"},
		Rules: []*apimodel.Rules{
			{
				BackendRefsRules: []*apimodel.BackendRefsRule{
					{Name: "svc-a", Kind: apimodel.Service, Port: 80},
					{Name: "svc-b", Kind: apimodel.Service, Port: 8080},
					{Name: "svc-a", Kind: apimodel.Service, Port: 80},
				},
			},
		},
	}

	events, err := (&GatewayAction{}).createGatewayHTTPRouteDeleteEvents(route, "alice")

	if err != nil {
		t.Fatalf("createGatewayHTTPRouteDeleteEvents returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if len(eventDao.events) != 2 {
		t.Fatalf("expected 2 persisted events, got %d", len(eventDao.events))
	}
	for _, event := range eventDao.events {
		if event.Target != dbmodel.TargetTypeService {
			t.Fatalf("event target = %s, expected %s", event.Target, dbmodel.TargetTypeService)
		}
		if event.OptType != "delete-gateway-http-route" {
			t.Fatalf("event opt_type = %s, expected delete-gateway-http-route", event.OptType)
		}
		if event.UserName != "alice" {
			t.Fatalf("event user = %s, expected alice", event.UserName)
		}
		if event.SynType != dbmodel.SYNEVENTTYPE {
			t.Fatalf("event syn_type = %d, expected %d", event.SynType, dbmodel.SYNEVENTTYPE)
		}
	}
	if eventDao.events[0].TargetID != "component-a" || eventDao.events[1].TargetID != "component-b" {
		t.Fatalf("unexpected event target IDs: %s, %s", eventDao.events[0].TargetID, eventDao.events[1].TargetID)
	}
}
