package controller

import (
	"context"
	"net/http"

	"github.com/goodrain/rainbond/config/configs"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
)

// RegisterConsoleReferenceWriter accepts only an authenticated Console protocol
// announcement. Runtime identities come from Kubernetes, never the request.
func (h *CleanupCoordinationHandler) RegisterConsoleReferenceWriter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pod      string `json:"pod"`
		PodUID   string `json:"pod_uid"`
		Protocol string `json:"protocol"`
	}
	if !coordinationDecode(w, r, &body) {
		return
	}
	if body.Pod == "" || body.PodUID == "" || body.Protocol != guard.ReferenceWriterProtocol {
		httputil.ReturnError(r, w, 400, "UNSUPPORTED_WRITER_PROTOCOL")
		return
	}
	if h.inspectConsoleWriter == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	observed, err := h.inspectConsoleWriter(r.Context(), body.Pod, body.PodUID)
	if err != nil || observed.Role != "console" || observed.Protocol != guard.ReferenceWriterProtocol || observed.PodName != body.Pod || observed.PodUID != body.PodUID {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	if err := guard.RegisterReferenceWriter(h.database(), observed); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}
func inspectSystemConsoleWriter(ctx context.Context, pod, uid string) (guard.ReferenceWriter, error) {
	component, configuration := k8s.Default(), configs.Default()
	if component == nil || component.Clientset == nil || configuration.PublicConfig == nil {
		return guard.ReferenceWriter{}, guard.ErrCoordinationUnavailable
	}
	return kubeidentity.InspectReferenceWriter(ctx, component.Clientset, configuration.PublicConfig.RbdNamespace, pod, uid, "console")
}
