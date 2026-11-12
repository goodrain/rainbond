package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
)

type forbiddenVMLoginDatabase struct{ db.Manager }

// capability_id: rainbond.vm-runtime.non-root-login-boundary
func TestVMRootSSHUsernameReturns400BeforeDatabaseAccess(t *testing.T) {
	db.SetTestManager(forbiddenVMLoginDatabase{})
	defer db.SetTestManager(nil)
	for _, value := range []string{"root", " root ", `"root"`} {
		for _, method := range []string{http.MethodPost, http.MethodPut, "SERVICE"} {
			t.Run(method+"/"+value, func(t *testing.T) {
				defer func() {
					if recover() != nil {
						t.Fatal("root login reached database access before rejection")
					}
				}()
				attribute := apimodel.ComponentK8sAttribute{Name: "vm_ssh_username", SaveType: "string", AttributeValue: value}
				var payload any = attribute
				if method == "SERVICE" {
					payload = apimodel.ServiceStruct{AppID: "app-vm", ComponentK8sAttributes: []apimodel.ComponentK8sAttribute{attribute}}
				}
				body, err := json.Marshal(payload)
				if err != nil {
					t.Fatal("test input could not encode")
				}
				request := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services", bytes.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(request.Context(), ctxutil.ContextKey("tenant_id"), "tenant-vm")
				ctx = context.WithValue(ctx, ctxutil.ContextKey("service_id"), "service-vm")
				request = request.WithContext(ctx)
				response := httptest.NewRecorder()
				controller := &K8sAttributeController{}
				switch method {
				case http.MethodPost:
					controller.createK8sAttributes(response, request)
				case http.MethodPut:
					controller.updateK8sAttributes(response, request)
				default:
					(&TenantStruct{}).CreateService(response, request)
				}
				if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "non-root") {
					t.Fatalf("root login status=%d; expected explicit non-root 400", response.Code)
				}
			})
		}
	}
}
