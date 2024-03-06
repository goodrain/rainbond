package apigateway

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/util/bcode"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	v1k8s "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"sigs.k8s.io/yaml"
)

// GetCert -
func (g Struct) GetCert(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	c := k8s.Default().ApiSixClient.ApisixV2()
	list, err := c.ApisixTlses(tenant.Namespace).List(r.Context(), v1.ListOptions{})
	if err != nil {
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFound)
		return
	}
	httputil.ReturnSuccess(r, w, list.Items)
}

// CreateCert -
func (g Struct) CreateCert(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	name := chi.URLParam(r, "name")
	clientset := handler.GetAPIGatewayHandler().GetK8sClient()

	tlsCert, err := clientset.CoreV1().Secrets(tenant.Namespace).Get(r.Context(), name, v1.GetOptions{})
	if err != nil {
		logrus.Errorf("get cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorK8sGetSecret)
		return
	}
	hosts, err := getCertificateDomains(tlsCert)
	if err != nil {
		logrus.Errorf("get cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorK8sGetSecret)
		return
	}

	c := k8s.Default().ApiSixClient.ApisixV2()
	create, err := c.ApisixTlses(tenant.Namespace).Create(r.Context(), &v2.ApisixTls{
		TypeMeta: v1.TypeMeta{
			Kind:       ApisixTLS,
			APIVersion: APIVersion,
		},
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "rbd",
			Name:         name,
		},
		Spec: &v2.ApisixTlsSpec{
			IngressClassName: "apisix",
			Hosts:            hosts,
			Secret: v2.ApisixSecret{
				Name:      name,
				Namespace: tenant.Namespace,
			},
		},
	}, v1.CreateOptions{})
	if err == nil {
		httputil.ReturnSuccess(r, w, marshalApisixTlses(create))
		return
	}

	get, err := c.ApisixTlses(tenant.Namespace).Get(r.Context(), name, v1.GetOptions{})
	if err != nil {
		logrus.Errorf("get cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrCertNotFound)
		return
	}

	get.Spec.Secret.Namespace = tenant.Namespace
	get.Spec.Secret.Name = name

	update, err := c.ApisixTlses(tenant.Namespace).Update(r.Context(), get, v1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("update cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorAPISixCertUpdateError)
		return
	}
	httputil.ReturnSuccess(r, w, marshalApisixTlses(update))

}

func marshalApisixTlses(r *v2.ApisixTls) map[string]interface{} {
	r.TypeMeta.Kind = "ApisixTls"
	r.TypeMeta.APIVersion = "apisix.apache.org/v2"
	r.ObjectMeta.ManagedFields = nil
	resp := make(map[string]interface{})
	contentBytes, _ := yaml.Marshal(r)
	resp["name"] = r.Name
	resp["kind"] = r.TypeMeta.Kind
	resp["content"] = string(contentBytes)
	return resp
}

func getCertificateDomains(tlsCert *v1k8s.Secret) ([]v2.HostType, error) {
	// Decode the certificate and private key from base64
	certData, certExists := tlsCert.Data["tls.crt"]
	keyData, keyExists := tlsCert.Data["tls.key"]

	if !certExists || !keyExists {
		return nil, fmt.Errorf("TLS certificate or key not found in the secret")
	}

	certBlock, _ := pem.Decode(certData)
	keyBlock, _ := pem.Decode(keyData)

	if certBlock == nil || keyBlock == nil {
		return nil, fmt.Errorf("failed to decode PEM block from certificate or private key")
	}

	// Parse the certificate to get the domains
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %v", err)
	}

	// Use a map to store unique domains
	uniqueDomains := make(map[v2.HostType]struct{})

	// Add the Common Name (CN) to unique domains
	uniqueDomains[v2.HostType(cert.Subject.CommonName)] = struct{}{}

	// Add Subject Alternative Names (SANs) to unique domains
	for _, dnsName := range cert.DNSNames {
		uniqueDomains[v2.HostType(dnsName)] = struct{}{}
	}

	// Convert the map to a slice
	var domains []v2.HostType
	for domain := range uniqueDomains {
		if domain != "" {
			domains = append(domains, domain)
		}
	}
	return domains, nil
}

// UpdateCert -
func (g Struct) UpdateCert(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	c := k8s.Default().ApiSixClient.ApisixV2()
	get, err := c.ApisixTlses(tenant.Namespace).Get(r.Context(), r.URL.Query().Get("certName"), v1.GetOptions{})
	if err != nil {
		logrus.Errorf("get cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorAPISixCertUpdateError)
		return
	}

	get.Spec.Secret.Namespace = tenant.Namespace
	get.Spec.Secret.Name = chi.URLParam(r, "name")

	update, err := c.ApisixTlses(tenant.Namespace).Update(r.Context(), get, v1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("update cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorAPISixCertUpdateError)
		return
	}
	httputil.ReturnSuccess(r, w, update.Spec)

}

// DeleteCert -
func (g Struct) DeleteCert(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	name := chi.URLParam(r, "name")
	c := k8s.Default().ApiSixClient.ApisixV2()
	err := c.ApisixTlses(tenant.Namespace).Delete(r.Context(), name, v1.DeleteOptions{})

	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("delete cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorAPISixDeleteCert)
		return
	}

	clientset := handler.GetAPIGatewayHandler().GetK8sClient()
	err = clientset.CoreV1().Secrets(tenant.Namespace).Delete(r.Context(), name, v1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("delete cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorK8sDeleteSecret)
		return
	}

	httputil.ReturnSuccess(r, w, nil)
}

// AutoCreateCert
func (g Struct) AutoCreateCert(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	domain := r.URL.Query().Get("domain")
	err := handler.GetAPIGatewayHandler().CreateCert(tenant.Namespace, r.URL.Query().Get("domain"))
	if err != nil {
		return
	}

	c := k8s.Default().ApiSixClient.ApisixV2()
	create, err := c.ApisixTlses(tenant.Namespace).Create(r.Context(), &v2.ApisixTls{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "rbd",
			Name:         domain,
		},
		Spec: &v2.ApisixTlsSpec{
			IngressClassName: "apisix",
			Hosts: []v2.HostType{
				v2.HostType(domain),
			},
			Secret: v2.ApisixSecret{
				Name:      domain,
				Namespace: tenant.Namespace,
			},
		},
	}, v1.CreateOptions{})
	if err != nil {
		logrus.Errorf("create cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorAPISixCreateCert)
		return
	}
	httputil.ReturnSuccess(r, w, create)
}
