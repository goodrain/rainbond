package apigateway

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	apimodel "github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

var getAPIGatewayHandler = handler.GetAPIGatewayHandler

// ListGatewayJWTConsumers lists JWT Consumers visible to the requested app.
// Credential material is projected out by the handler before it reaches this
// controller.
func (g Struct) ListGatewayJWTConsumers(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	consumers, err := getAPIGatewayHandler().ListGatewayJWTConsumers(r.Context(), tenant.Namespace, r.URL.Query().Get("appID"))
	if err != nil {
		returnGatewayJWTConsumerError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, consumers)
}

// CreateGatewayJWTConsumer creates a Rainbond-managed Consumer and credential.
func (g Struct) CreateGatewayJWTConsumer(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	var req apimodel.GatewayJWTConsumerRequest
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		return
	}
	consumer, err := getAPIGatewayHandler().CreateGatewayJWTConsumer(r.Context(), tenant.Namespace, r.URL.Query().Get("appID"), &req)
	if err != nil {
		returnGatewayJWTConsumerError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, consumer)
}

// RotateGatewayJWTConsumer replaces a managed Consumer's credential.
func (g Struct) RotateGatewayJWTConsumer(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	var req apimodel.GatewayJWTConsumerCredential
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		return
	}
	consumer, err := getAPIGatewayHandler().RotateGatewayJWTConsumer(r.Context(), tenant.Namespace, chi.URLParam(r, "name"), &req)
	if err != nil {
		returnGatewayJWTConsumerError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, consumer)
}

// DeleteGatewayJWTConsumer deletes an unbound Rainbond-managed Consumer.
func (g Struct) DeleteGatewayJWTConsumer(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	if err := getAPIGatewayHandler().DeleteGatewayJWTConsumer(r.Context(), tenant.Namespace, chi.URLParam(r, "name")); err != nil {
		returnGatewayJWTConsumerError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

func returnGatewayJWTConsumerError(r *http.Request, w http.ResponseWriter, err error) {
	status := gatewayJWTConsumerErrorStatus(err)
	if status == http.StatusInternalServerError {
		// Do not log the wrapped error here: upstream errors may include Secret
		// field values. The public response and this log remain credential-safe.
		logrus.Errorf("gateway JWT consumer operation failed, path: %s", r.URL.Path)
	}
	httputil.ReturnError(r, w, status, gatewayJWTConsumerPublicError(err))
}

func gatewayJWTConsumerErrorStatus(err error) int {
	switch {
	case errors.Is(err, handler.ErrGatewayJWTConsumerInvalidName),
		errors.Is(err, handler.ErrGatewayJWTConsumerInvalidCredential),
		errors.Is(err, handler.ErrGatewayJWTAuthInvalidConfig),
		errors.Is(err, handler.ErrGatewayJWTConsumerRequired),
		errors.Is(err, handler.ErrGatewayJWTConsumerNotJWT),
		k8serrors.IsBadRequest(err),
		k8serrors.IsInvalid(err):
		return http.StatusBadRequest
	case errors.Is(err, handler.ErrGatewayJWTConsumerNotManaged),
		errors.Is(err, handler.ErrGatewayJWTConsumerAppMismatch),
		k8serrors.IsForbidden(err):
		return http.StatusForbidden
	case k8serrors.IsNotFound(err):
		return http.StatusNotFound
	case errors.Is(err, handler.ErrGatewayJWTConsumerInUse),
		k8serrors.IsAlreadyExists(err),
		k8serrors.IsConflict(err):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func gatewayJWTConsumerPublicError(err error) string {
	switch {
	case errors.Is(err, handler.ErrGatewayJWTConsumerInvalidName):
		return handler.ErrGatewayJWTConsumerInvalidName.Error()
	case errors.Is(err, handler.ErrGatewayJWTConsumerInvalidCredential):
		return handler.ErrGatewayJWTConsumerInvalidCredential.Error()
	case errors.Is(err, handler.ErrGatewayJWTAuthInvalidConfig):
		return handler.ErrGatewayJWTAuthInvalidConfig.Error()
	case errors.Is(err, handler.ErrGatewayJWTConsumerRequired):
		return handler.ErrGatewayJWTConsumerRequired.Error()
	case errors.Is(err, handler.ErrGatewayJWTConsumerNotJWT):
		return handler.ErrGatewayJWTConsumerNotJWT.Error()
	case errors.Is(err, handler.ErrGatewayJWTConsumerNotManaged):
		return handler.ErrGatewayJWTConsumerNotManaged.Error()
	case errors.Is(err, handler.ErrGatewayJWTConsumerAppMismatch):
		return handler.ErrGatewayJWTConsumerAppMismatch.Error()
	case errors.Is(err, handler.ErrGatewayJWTConsumerInUse):
		return handler.ErrGatewayJWTConsumerInUse.Error()
	case k8serrors.IsForbidden(err):
		return "gateway JWT consumer operation is forbidden"
	case k8serrors.IsNotFound(err):
		return "gateway JWT consumer not found"
	case k8serrors.IsAlreadyExists(err):
		return "gateway JWT consumer already exists"
	case k8serrors.IsConflict(err):
		return "gateway JWT consumer has changed; retry the operation"
	case k8serrors.IsBadRequest(err), k8serrors.IsInvalid(err):
		return "gateway JWT consumer request is invalid"
	default:
		return "gateway JWT consumer operation failed"
	}
}
