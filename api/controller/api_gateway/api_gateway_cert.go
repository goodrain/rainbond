package api_gateway

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/util/bcode"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	v1k8s "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

func (g APIGatewayStruct) GetCert(w http.ResponseWriter, r *http.Request) {
	c := handler.GetAPIGatewayHandler().GetClient().ApisixV2()
	list, err := c.ApisixTlses(r.URL.Query().Get("namespace")).List(r.Context(), v1.ListOptions{})
	if err != nil {
		httputil.ReturnBcodeError(r, w, bcode.ErrCertNotFount)
		return
	}
	resp := make([]responseBody, 0)
	for _, item := range list.Items {
		resp = append(resp, responseBody{
			Name: item.Name,
			Body: item.Spec,
		})
	}

	httputil.ReturnSuccess(r, w, resp)
}

func (g APIGatewayStruct) CreateCert(w http.ResponseWriter, r *http.Request) {
	certName := r.URL.Query().Get("certName")
	clientset := handler.GetAPIGatewayHandler().GetK8sClient()

	tlsCert, err := clientset.CoreV1().Secrets(r.URL.Query().Get("namespace")).Get(r.Context(), certName, v1.GetOptions{})
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

	c := handler.GetAPIGatewayHandler().GetClient().ApisixV2()
	create, err := c.ApisixTlses(r.URL.Query().Get("namespace")).Create(r.Context(), &v2.ApisixTls{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "rbd",
			Name:         certName,
		},
		Spec: &v2.ApisixTlsSpec{
			IngressClassName: "apisix",
			Hosts:            hosts,
			Secret: v2.ApisixSecret{
				Name:      certName,
				Namespace: r.URL.Query().Get("namespace"),
			},
		},
	}, v1.CreateOptions{})
	if err != nil {
		httputil.ReturnBcodeError(r, w, bcode.ErrorAPISixCreateCert)
		return
	}
	httputil.ReturnSuccess(r, w, create)
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
		domains = append(domains, domain)
	}
	return domains, nil
}

func (g APIGatewayStruct) UpdateCert(w http.ResponseWriter, r *http.Request) {
	c := handler.GetAPIGatewayHandler().GetClient().ApisixV2()
	get, err := c.ApisixTlses(r.URL.Query().Get("namespace")).Get(r.Context(), r.URL.Query().Get("certName"), v1.GetOptions{})
	if err != nil {
		logrus.Errorf("get cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorAPISixCertNotFount)
		return
	}

	get.Spec.Secret.Namespace = r.URL.Query().Get("namespace")
	get.Spec.Secret.Name = chi.URLParam(r, "name")

	update, err := c.ApisixTlses(r.URL.Query().Get("namespace")).Update(r.Context(), get, v1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("update cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorAPISixCertUpdateError)
		return
	}
	httputil.ReturnSuccess(r, w, update.Spec)

}

func (g APIGatewayStruct) DeleteCert(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	c := handler.GetAPIGatewayHandler().GetClient().ApisixV2()
	err := c.ApisixTlses(r.URL.Query().Get("namespace")).Delete(r.Context(), name, v1.DeleteOptions{})

	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("delete cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorAPISixDeleteCert)
		return
	}

	clientset := handler.GetAPIGatewayHandler().GetK8sClient()
	err = clientset.CoreV1().Secrets(r.URL.Query().Get("namespace")).Delete(r.Context(), name, v1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("delete cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorK8sDeleteSecret)
		return
	}

	httputil.ReturnSuccess(r, w, nil)
}
