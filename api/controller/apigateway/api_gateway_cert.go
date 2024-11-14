package apigateway

import (
	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/api/util/bcode"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
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

	tlsCert, err := k8s.Default().Clientset.CoreV1().Secrets(tenant.Namespace).Get(r.Context(), name, v1.GetOptions{})
	if err != nil {
		logrus.Errorf("get cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorK8sGetSecret)
		return
	}
	hosts, err := util.GetCertificateDomains(tlsCert)
	if err != nil {
		logrus.Errorf("get cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorK8sGetSecret)
		return
	}

	c := k8s.Default().ApiSixClient.ApisixV2()
	create, err := c.ApisixTlses(tenant.Namespace).Create(r.Context(), &v2.ApisixTls{
		TypeMeta: v1.TypeMeta{
			Kind:       util.ApisixTLS,
			APIVersion: util.APIVersion,
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

	err = k8s.Default().Clientset.CoreV1().Secrets(tenant.Namespace).Delete(r.Context(), name, v1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("delete cert error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrorK8sDeleteSecret)
		return
	}

	httputil.ReturnSuccess(r, w, nil)
}

// AutoCreateCert -
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
