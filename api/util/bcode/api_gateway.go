package bcode

import "net/http"

const (
	RouteNotFount = iota + 50001
	RouteUpdateError
	RouteCreateError
	RouteDeleteError

	ServiceNotFount
	ServiceUpdateError
	ServiceCreateError
	ServiceDeleteError

	CertNotFount
	K8sSecretCreateError
	K8sGetSecretError
	K8sDeleteSecretError
	APISixCreateCertError
	APISixDeleteCertError

	APISixCertNotFount
	APISixCertUpdateError
)

var (
	ErrRouteNotFount = newByMessage(http.StatusNotFound, RouteNotFount, "route not found")
	ErrRouteUpdate   = newByMessage(http.StatusBadRequest, RouteUpdateError, "route update error")
	ErrRouteCreate   = newByMessage(http.StatusBadRequest, RouteCreateError, "route create error")
	ErrRouteDelete   = newByMessage(http.StatusBadRequest, RouteDeleteError, "route delete error")

	ErrServiceNotFount = newByMessage(http.StatusNotFound, ServiceNotFount, "service not found")
	ErrServiceUpdate   = newByMessage(http.StatusBadRequest, ServiceUpdateError, "service update error")
	ErrServiceCreate   = newByMessage(http.StatusBadRequest, ServiceCreateError, "service create error")
	ErrServiceDelete   = newByMessage(http.StatusBadRequest, ServiceDeleteError, "service delete error")

	ErrCertNotFount            = newByMessage(http.StatusNotFound, CertNotFount, "cert not found")
	ErrorK8sSecretCreate       = newByMessage(http.StatusBadRequest, K8sSecretCreateError, "k8s secret create error")
	ErrorK8sGetSecret          = newByMessage(http.StatusBadRequest, K8sGetSecretError, "k8s get secret error")
	ErrorK8sDeleteSecret       = newByMessage(http.StatusBadRequest, K8sDeleteSecretError, "k8s delete secret error")
	ErrorAPISixCreateCert      = newByMessage(http.StatusBadRequest, APISixCreateCertError, "apisix create cert error")
	ErrorAPISixDeleteCert      = newByMessage(http.StatusBadRequest, APISixDeleteCertError, "apisix delete cert error")
	ErrorAPISixCertNotFount    = newByMessage(http.StatusNotFound, APISixCertNotFount, "apisix cert not found")
	ErrorAPISixCertUpdateError = newByMessage(http.StatusBadRequest, APISixCertUpdateError, "apisix cert update error")
)
