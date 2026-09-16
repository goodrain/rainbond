package handler

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

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
	return gatewayCertificateRequestForDomains(t, name, namespace, []string{domain})
}

func gatewayCertificateRequestForDomains(t *testing.T, name, namespace string, domains []string) *apimodel.GatewayCertificate {
	t.Helper()
	if len(domains) == 0 {
		t.Fatal("at least one certificate domain is required")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate certificate key: %v", err)
	}
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate certificate serial: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: domains[0]},
		DNSNames:              domains,
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal certificate key: %v", err)
	}

	return &apimodel.GatewayCertificate{
		Name:        name,
		Namespace:   namespace,
		Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})),
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

func testCertificatePEM(t *testing.T, commonName string, isCA bool, notAfter time.Time) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate certificate key: %v", err)
	}
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate certificate serial: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
		KeyUsage:              x509.KeyUsageDigitalSignature,
	}
	if isCA {
		template.KeyUsage |= x509.KeyUsageCertSign
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// capability_id: rainbond.gateway.client-ca-lifecycle
func TestGatewayClientCALifecycle(t *testing.T) {
	const (
		namespace = "tenant-ns"
		name      = "rbd-client-ca-certificate-id"
	)

	t.Run("creates updates and lists a valid client CA", func(t *testing.T) {
		firstCertificate := testCertificatePEM(t, "first-client-ca", true, time.Now().Add(24*time.Hour))
		updatedCertificate := testCertificatePEM(t, "updated-client-ca", true, time.Now().Add(48*time.Hour))
		kubeClient := k8sfake.NewSimpleClientset()
		apisixClient := apisixfake.NewSimpleClientset()
		action := gatewayActionForCertificateTest(kubeClient, apisixClient)

		if err := action.AddGatewayClientCA(namespace, &apimodel.GatewayClientCA{
			Name: name, Certificate: firstCertificate,
		}); err != nil {
			t.Fatalf("add client CA: %v", err)
		}
		secret, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get client CA secret: %v", err)
		}
		if got := string(secret.Data[corev1.ServiceAccountRootCAKey]); got != firstCertificate {
			t.Fatal("client CA secret does not contain the submitted certificate")
		}
		if _, ok := secret.Data[corev1.TLSPrivateKeyKey]; ok {
			t.Fatal("client CA secret must not contain a private key")
		}

		if err := action.UpdateGatewayClientCA(namespace, &apimodel.GatewayClientCA{
			Name: name, Certificate: updatedCertificate,
		}); err != nil {
			t.Fatalf("update client CA: %v", err)
		}
		secret, err = kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get updated client CA secret: %v", err)
		}
		if got := string(secret.Data[corev1.ServiceAccountRootCAKey]); got != updatedCertificate {
			t.Fatal("updated client CA secret does not contain the submitted certificate")
		}

		_, err = apisixClient.ApisixV2().ApisixTlses(namespace).Create(context.Background(), &v2.ApisixTls{
			ObjectMeta: metav1.ObjectMeta{Name: "api-tls", Namespace: namespace},
			Spec: &v2.ApisixTlsSpec{
				Hosts:  []v2.HostType{"api.example.com"},
				Secret: v2.ApisixSecret{Name: "server-cert", Namespace: namespace},
				Client: &v2.ApisixMutualTlsClientConfig{
					CASecret: v2.ApisixSecret{Name: name, Namespace: namespace},
					Depth:    1,
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("create referencing ApisixTls: %v", err)
		}

		statuses, err := action.ListGatewayClientCAs(namespace)
		if err != nil {
			t.Fatalf("list client CAs: %v", err)
		}
		if len(statuses) != 1 || statuses[0].Name != name || !reflect.DeepEqual(statuses[0].BoundDomains, []string{"api.example.com"}) {
			t.Fatalf("client CA statuses = %#v", statuses)
		}

		if err := action.DeleteGatewayClientCA(namespace, name); err == nil {
			t.Fatal("expected referenced client CA deletion to fail")
		}
		if _, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
			t.Fatalf("referenced client CA secret was deleted: %v", err)
		}

		if err := apisixClient.ApisixV2().ApisixTlses(namespace).Delete(context.Background(), "api-tls", metav1.DeleteOptions{}); err != nil {
			t.Fatalf("delete referencing ApisixTls: %v", err)
		}
		if err := action.DeleteGatewayClientCA(namespace, name); err != nil {
			t.Fatalf("delete unreferenced client CA: %v", err)
		}
		if _, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{}); !k8serrors.IsNotFound(err) {
			t.Fatalf("client CA secret still exists, error: %v", err)
		}
	})

	tests := []struct {
		name        string
		certificate string
	}{
		{name: "malformed", certificate: "not a certificate"},
		{name: "non CA", certificate: testCertificatePEM(t, "client", false, time.Now().Add(24*time.Hour))},
		{name: "expired", certificate: testCertificatePEM(t, "expired-ca", true, time.Now().Add(-time.Minute))},
	}
	for _, tt := range tests {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			kubeClient := k8sfake.NewSimpleClientset()
			action := gatewayActionForCertificateTest(kubeClient, apisixfake.NewSimpleClientset())
			if err := action.AddGatewayClientCA(namespace, &apimodel.GatewayClientCA{
				Name: name, Certificate: tt.certificate,
			}); err == nil {
				t.Fatalf("expected %s certificate to be rejected", tt.name)
			}
			if _, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{}); !k8serrors.IsNotFound(err) {
				t.Fatalf("invalid client CA created a secret, error: %v", err)
			}
		})
	}
}

func tlsHostsForTest(tls *v2.ApisixTls) []string {
	hosts := make([]string, 0, len(tls.Spec.Hosts))
	for _, host := range tls.Spec.Hosts {
		hosts = append(hosts, string(host))
	}
	sort.Strings(hosts)
	return hosts
}

// capability_id: rainbond.gateway.domain-mtls
func TestGatewayDomainMTLS(t *testing.T) {
	const (
		namespace = "tenant-ns"
		caName    = "rbd-client-ca-certificate-id"
	)
	validCA := testCertificatePEM(t, "client-ca", true, time.Now().Add(24*time.Hour))
	clientCASecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      caName,
			Namespace: namespace,
			Labels:    map[string]string{gatewayClientCASecretLabel: gatewayClientCASecretLabelValue},
		},
		Data: map[string][]byte{corev1.ServiceAccountRootCAKey: []byte(validCA)},
	}

	t.Run("configures and disables a single exact domain", func(t *testing.T) {
		tls := &v2.ApisixTls{
			ObjectMeta: metav1.ObjectMeta{Name: "server-tls", Namespace: namespace},
			Spec: &v2.ApisixTlsSpec{
				Hosts:  []v2.HostType{"api.example.com"},
				Secret: v2.ApisixSecret{Name: "server-secret", Namespace: namespace},
			},
		}
		action := gatewayActionForCertificateTest(k8sfake.NewSimpleClientset(clientCASecret.DeepCopy()), apisixfake.NewSimpleClientset(tls))

		if err := action.ConfigureGatewayDomainMTLS(namespace, &apimodel.GatewayDomainMTLS{
			Domain: "api.example.com", ClientCASecretName: caName,
		}); err != nil {
			t.Fatalf("configure domain mTLS: %v", err)
		}
		updated, err := action.apisixClient.ApisixV2().ApisixTlses(namespace).Get(context.Background(), "server-tls", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get configured ApisixTls: %v", err)
		}
		if updated.Spec.Client == nil || updated.Spec.Client.CASecret.Name != caName || updated.Spec.Client.Depth != 1 {
			t.Fatalf("mTLS client config = %#v", updated.Spec.Client)
		}
		statuses, err := action.GetGatewayDomainMTLS(namespace, []string{"api.example.com", "other.example.com"})
		if err != nil {
			t.Fatalf("get domain mTLS status: %v", err)
		}
		if len(statuses) != 2 || !statuses[0].Enabled || statuses[0].ClientCASecretName != caName || statuses[1].Enabled {
			t.Fatalf("domain mTLS statuses = %#v", statuses)
		}

		if err := action.DisableGatewayDomainMTLS(namespace, "api.example.com"); err != nil {
			t.Fatalf("disable domain mTLS: %v", err)
		}
		updated, err = action.apisixClient.ApisixV2().ApisixTlses(namespace).Get(context.Background(), "server-tls", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get disabled ApisixTls: %v", err)
		}
		if updated.Spec.Client != nil {
			t.Fatalf("mTLS client config was not removed: %#v", updated.Spec.Client)
		}
	})

	t.Run("splits and restores one domain from a multi host certificate", func(t *testing.T) {
		tls := &v2.ApisixTls{
			ObjectMeta: metav1.ObjectMeta{Name: "server-tls", Namespace: namespace},
			Spec: &v2.ApisixTlsSpec{
				Hosts:  []v2.HostType{"api.example.com", "www.example.com"},
				Secret: v2.ApisixSecret{Name: "server-secret", Namespace: namespace},
			},
		}
		action := gatewayActionForCertificateTest(k8sfake.NewSimpleClientset(clientCASecret.DeepCopy()), apisixfake.NewSimpleClientset(tls))

		if err := action.ConfigureGatewayDomainMTLS(namespace, &apimodel.GatewayDomainMTLS{
			Domain: "api.example.com", ClientCASecretName: caName,
		}); err != nil {
			t.Fatalf("configure split domain mTLS: %v", err)
		}
		list, err := action.apisixClient.ApisixV2().ApisixTlses(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("list split ApisixTls resources: %v", err)
		}
		if len(list.Items) != 2 {
			t.Fatalf("ApisixTls resource count = %d, expected 2", len(list.Items))
		}
		base, err := action.apisixClient.ApisixV2().ApisixTlses(namespace).Get(context.Background(), "server-tls", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get base ApisixTls: %v", err)
		}
		if !reflect.DeepEqual(tlsHostsForTest(base), []string{"www.example.com"}) {
			t.Fatalf("base hosts = %#v", base.Spec.Hosts)
		}
		var split *v2.ApisixTls
		for i := range list.Items {
			candidate := &list.Items[i]
			if candidate.Name != "server-tls" {
				split = candidate
			}
		}
		if split == nil || !reflect.DeepEqual(tlsHostsForTest(split), []string{"api.example.com"}) || split.Spec.Client == nil {
			t.Fatalf("split ApisixTls = %#v", split)
		}
		if split.Spec.Secret != tls.Spec.Secret {
			t.Fatalf("split server secret = %#v, expected %#v", split.Spec.Secret, tls.Spec.Secret)
		}

		if err := action.DisableGatewayDomainMTLS(namespace, "api.example.com"); err != nil {
			t.Fatalf("disable split domain mTLS: %v", err)
		}
		list, err = action.apisixClient.ApisixV2().ApisixTlses(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("list restored ApisixTls resources: %v", err)
		}
		if len(list.Items) != 1 || list.Items[0].Name != "server-tls" {
			t.Fatalf("restored ApisixTls resources = %#v", list.Items)
		}
		if !reflect.DeepEqual(tlsHostsForTest(&list.Items[0]), []string{"api.example.com", "www.example.com"}) {
			t.Fatalf("restored base hosts = %#v", list.Items[0].Spec.Hosts)
		}
	})

	t.Run("creates a specific override for a wildcard certificate", func(t *testing.T) {
		tls := &v2.ApisixTls{
			ObjectMeta: metav1.ObjectMeta{Name: "wildcard-tls", Namespace: namespace},
			Spec: &v2.ApisixTlsSpec{
				Hosts:  []v2.HostType{"*.example.com"},
				Secret: v2.ApisixSecret{Name: "wildcard-secret", Namespace: namespace},
			},
		}
		action := gatewayActionForCertificateTest(k8sfake.NewSimpleClientset(clientCASecret.DeepCopy()), apisixfake.NewSimpleClientset(tls))

		if err := action.ConfigureGatewayDomainMTLS(namespace, &apimodel.GatewayDomainMTLS{
			Domain: "api.example.com", ClientCASecretName: caName,
		}); err != nil {
			t.Fatalf("configure wildcard domain mTLS: %v", err)
		}
		list, err := action.apisixClient.ApisixV2().ApisixTlses(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("list wildcard ApisixTls resources: %v", err)
		}
		if len(list.Items) != 2 {
			t.Fatalf("wildcard ApisixTls resource count = %d, expected 2", len(list.Items))
		}
		base, err := action.apisixClient.ApisixV2().ApisixTlses(namespace).Get(context.Background(), "wildcard-tls", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get wildcard base ApisixTls: %v", err)
		}
		if !reflect.DeepEqual(tlsHostsForTest(base), []string{"*.example.com"}) || base.Spec.Client != nil {
			t.Fatalf("wildcard base was modified: %#v", base.Spec)
		}

		if err := action.DisableGatewayDomainMTLS(namespace, "api.example.com"); err != nil {
			t.Fatalf("disable wildcard domain mTLS: %v", err)
		}
		list, err = action.apisixClient.ApisixV2().ApisixTlses(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("list wildcard resources after disable: %v", err)
		}
		if len(list.Items) != 1 || list.Items[0].Name != "wildcard-tls" {
			t.Fatalf("wildcard resources after disable = %#v", list.Items)
		}
	})

	t.Run("configures a wildcard domain", func(t *testing.T) {
		tls := &v2.ApisixTls{
			ObjectMeta: metav1.ObjectMeta{Name: "wildcard-tls", Namespace: namespace},
			Spec: &v2.ApisixTlsSpec{
				Hosts:  []v2.HostType{"*.example.com"},
				Secret: v2.ApisixSecret{Name: "wildcard-secret", Namespace: namespace},
			},
		}
		action := gatewayActionForCertificateTest(k8sfake.NewSimpleClientset(clientCASecret.DeepCopy()), apisixfake.NewSimpleClientset(tls))

		if err := action.ConfigureGatewayDomainMTLS(namespace, &apimodel.GatewayDomainMTLS{
			Domain: "*.example.com", ClientCASecretName: caName,
		}); err != nil {
			t.Fatalf("configure wildcard mTLS: %v", err)
		}
		updated, err := action.apisixClient.ApisixV2().ApisixTlses(namespace).Get(
			context.Background(), "wildcard-tls", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get configured wildcard ApisixTls: %v", err)
		}
		if updated.Spec.Client == nil || updated.Spec.Client.CASecret.Name != caName {
			t.Fatalf("wildcard mTLS client config = %#v", updated.Spec.Client)
		}
	})

	t.Run("rejects missing client CA and missing server TLS", func(t *testing.T) {
		action := gatewayActionForCertificateTest(k8sfake.NewSimpleClientset(), apisixfake.NewSimpleClientset())
		if err := action.ConfigureGatewayDomainMTLS(namespace, &apimodel.GatewayDomainMTLS{
			Domain: "api.example.com", ClientCASecretName: caName,
		}); err == nil {
			t.Fatal("expected missing client CA to fail")
		}

		action = gatewayActionForCertificateTest(k8sfake.NewSimpleClientset(clientCASecret.DeepCopy()), apisixfake.NewSimpleClientset())
		if err := action.ConfigureGatewayDomainMTLS(namespace, &apimodel.GatewayDomainMTLS{
			Domain: "api.example.com", ClientCASecretName: caName,
		}); err == nil {
			t.Fatal("expected missing server TLS to fail")
		}
	})
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

	t.Run("update keeps detached mTLS domains out of the base resource", func(t *testing.T) {
		req := gatewayCertificateRequestForDomains(t, name, namespace, []string{"api.example.com", "www.example.com"})
		baseTLS := &v2.ApisixTls{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: &v2.ApisixTlsSpec{
				Hosts:  []v2.HostType{"www.example.com"},
				Secret: v2.ApisixSecret{Name: name, Namespace: namespace},
			},
		}
		childTLS := &v2.ApisixTls{
			ObjectMeta: metav1.ObjectMeta{
				Name:      gatewayMTLSResourceName("api.example.com"),
				Namespace: namespace,
				Labels:    map[string]string{gatewayMTLSManagedLabel: "true"},
				Annotations: map[string]string{
					gatewayMTLSDomainAnnotation:   "api.example.com",
					gatewayMTLSParentAnnotation:   name,
					gatewayMTLSDetachedAnnotation: "true",
					gatewayMTLSInPlaceAnnotation:  "false",
				},
			},
			Spec: &v2.ApisixTlsSpec{
				Hosts:  []v2.HostType{"api.example.com"},
				Secret: v2.ApisixSecret{Name: name, Namespace: namespace},
				Client: gatewayMTLSClient(namespace, "client-ca"),
			},
		}
		action := gatewayActionForCertificateTest(
			k8sfake.NewSimpleClientset(),
			apisixfake.NewSimpleClientset(baseTLS, childTLS),
		)

		if err := action.UpdateGatewayCertificate(req); err != nil {
			t.Fatalf("update gateway certificate with detached mTLS domain: %v", err)
		}
		updatedBase, err := action.apisixClient.ApisixV2().ApisixTlses(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get updated base ApisixTls: %v", err)
		}
		if !reflect.DeepEqual(tlsHostsForTest(updatedBase), []string{"www.example.com"}) {
			t.Fatalf("updated base hosts = %#v, expected detached domain to stay excluded", updatedBase.Spec.Hosts)
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

	t.Run("delete removes managed mTLS resources that reference the server certificate", func(t *testing.T) {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		baseTLS := &v2.ApisixTls{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		childTLS := &v2.ApisixTls{
			ObjectMeta: metav1.ObjectMeta{
				Name:      gatewayMTLSResourceName("api.example.com"),
				Namespace: namespace,
				Labels:    map[string]string{gatewayMTLSManagedLabel: "true"},
				Annotations: map[string]string{
					gatewayMTLSDomainAnnotation: "api.example.com",
					gatewayMTLSParentAnnotation: name,
				},
			},
			Spec: &v2.ApisixTlsSpec{Secret: v2.ApisixSecret{Name: name, Namespace: namespace}},
		}
		action := gatewayActionForCertificateTest(
			k8sfake.NewSimpleClientset(secret),
			apisixfake.NewSimpleClientset(baseTLS, childTLS),
		)

		if err := action.DeleteGatewayCertificate(name, namespace); err != nil {
			t.Fatalf("delete gateway certificate with managed mTLS child: %v", err)
		}
		list, err := action.apisixClient.ApisixV2().ApisixTlses(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("list ApisixTls resources after certificate deletion: %v", err)
		}
		if len(list.Items) != 0 {
			t.Fatalf("orphan ApisixTls resources remain: %#v", list.Items)
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
