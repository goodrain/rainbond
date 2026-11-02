package apigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	apisixversioned "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// capability_id: rainbond.gateway.http-route-source-ip-validation
func TestCreateHTTPAPIRouteNormalizesRemoteAddrs(t *testing.T) {
	tests := []struct {
		name       string
		existing   bool
		addrs      []string
		want       []string
		wantStatus int
	}{
		{name: "create without restriction", wantStatus: http.StatusOK},
		{name: "create ignores empty address", addrs: []string{""}, wantStatus: http.StatusOK},
		{name: "clear existing restriction", existing: true, addrs: []string{""}, wantStatus: http.StatusOK},
		{name: "clear existing restriction with empty list", existing: true, addrs: []string{}, wantStatus: http.StatusOK},
		{name: "clear whitespace-only restriction", existing: true, addrs: []string{" ", "\t"}, wantStatus: http.StatusOK},
		{
			name: "preserve trimmed IP and CIDR entries", existing: true,
			addrs: []string{" 203.0.113.10 ", "", "192.0.2.0/24", "2001:db8::1", " 2001:db8::/32 ", "\t"},
			want:  []string{"203.0.113.10", "192.0.2.0/24", "2001:db8::1", "2001:db8::/32"}, wantStatus: http.StatusOK,
		},
		{name: "reject hostname on create", addrs: []string{"example.com"}, wantStatus: http.StatusBadRequest},
		{name: "reject invalid IP on update", existing: true, addrs: []string{"203.0.113.999"}, wantStatus: http.StatusBadRequest},
		{name: "reject invalid CIDR", existing: true, addrs: []string{"192.0.2.0/33"}, wantStatus: http.StatusBadRequest},
		{name: "reject invalid IPv6 CIDR", addrs: []string{"2001:db8::/129"}, wantStatus: http.StatusBadRequest},
		{name: "reject mixed valid and invalid entries", existing: true, addrs: []string{"203.0.113.10", "bad"}, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const routeName = "example.comp-ps-s"
			var saved *v2.ApisixRoute
			if tt.existing {
				saved = &v2.ApisixRoute{
					TypeMeta:   metav1.TypeMeta{APIVersion: "apisix.apache.org/v2", Kind: "ApisixRoute"},
					ObjectMeta: metav1.ObjectMeta{Name: routeName, Namespace: "default", Labels: map[string]string{}},
					Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{
						Name: "existing-rule", Match: v2.ApisixRouteHTTPMatch{
							Hosts: []string{"example.com"}, Paths: []string{"/*"}, RemoteAddrs: []string{"203.0.113.10"},
						},
					}}},
				}
			}
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case http.MethodPost:
					if saved != nil {
						w.WriteHeader(http.StatusConflict)
						_ = json.NewEncoder(w).Encode(metav1.Status{
							TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}, Status: metav1.StatusFailure,
							Reason: metav1.StatusReasonAlreadyExists, Code: http.StatusConflict,
						})
						return
					}
					fallthrough
				case http.MethodPut:
					var route v2.ApisixRoute
					if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
						t.Errorf("decode route: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					saved = &route
				case http.MethodGet:
				default:
					t.Errorf("unexpected Kubernetes request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				_ = json.NewEncoder(w).Encode(saved)
			}))
			t.Cleanup(server.Close)
			client, err := apisixversioned.NewForConfig(&rest.Config{Host: server.URL})
			require.NoError(t, err)
			k8s.New().ApiSixClient = client

			body, err := json.Marshal(v2.ApisixRouteHTTP{
				Match: v2.ApisixRouteHTTPMatch{Hosts: []string{"example.com"}, Paths: []string{"/*"}, RemoteAddrs: tt.addrs},
			})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/?appID=app", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(context.WithValue(req.Context(), ctxutil.ContextKey("tenant"), &dbmodel.Tenants{Namespace: "default"}))
			response := httptest.NewRecorder()
			Struct{}.CreateHTTPAPIRoute(response, req)

			require.Equal(t, tt.wantStatus, response.Code, response.Body.String())
			if tt.wantStatus == http.StatusBadRequest {
				assert.Zero(t, requests, "invalid source IP must not reach Kubernetes")
				assert.Contains(t, response.Body.String(), "source IP")
				if tt.existing {
					assert.Equal(t, []string{"203.0.113.10"}, saved.Spec.HTTP[0].Match.RemoteAddrs)
				}
				return
			}
			require.NotNil(t, saved)
			require.Len(t, saved.Spec.HTTP, 1)
			assert.Equal(t, tt.want, saved.Spec.HTTP[0].Match.RemoteAddrs)
			if tt.existing {
				assert.Equal(t, 3, requests, "existing rule must be read and updated")
				assert.Equal(t, "existing-rule", saved.Spec.HTTP[0].Name)
			}
		})
	}
}
