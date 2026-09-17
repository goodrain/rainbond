package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/goodrain/rainbond-operator/api/v1alpha1"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// capability_id: rainbond.cluster.upgrade-preserves-component-config
func TestUpgradePreservesComponentConfig(t *testing.T) {
	for _, tc := range []struct {
		name      string
		namespace string
		image     string
	}{
		{name: "new image", image: "example.invalid/rainbond:new"},
		{name: "same image", image: "example.invalid/rainbond:old"},
		{name: "custom namespace", namespace: "custom-system", image: "example.invalid/rainbond:new"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RBD_NAMESPACE", tc.namespace)
			namespace := tc.namespace
			if namespace == "" {
				namespace = "rbd-system"
			}
			// affinity and tolerations are valid CRD fields missing from the vendored Go type.
			stored := []byte(`{
				"apiVersion":"rainbond.io/v1alpha1","kind":"RbdComponent",
				"metadata":{"name":"rbd-app-ui","namespace":"` + namespace + `","resourceVersion":"42",
					"labels":{"custom":"keep"},"annotations":{"custom":"keep"}},
				"spec":{"image":"example.invalid/rainbond:old","replicas":3,"priorityComponent":false,
					"imagePullPolicy":"IfNotPresent","args":["--custom"],
					"env":[{"name":"DB_TYPE","value":"mysql"}],
					"resources":{"requests":{"cpu":"100m"}},
					"volumes":[{"name":"custom","emptyDir":{}}],
					"volumeMounts":[{"name":"custom","mountPath":"/custom"}],
					"affinity":{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{
						"nodeSelectorTerms":[{"matchFields":[{"key":"metadata.name","operator":"In","values":["custom-node"]}]}]}}},
					"tolerations":[{"key":"node.kubernetes.io/unschedulable","operator":"Exists","effect":"NoSchedule"}]},
				"status":{"conditions":[{"type":"ClusterConfigCompeleted","status":"True","reason":"ConfigCompleted"}]}
			}`)
			var expected map[string]interface{}
			if err := json.Unmarshal(stored, &expected); err != nil {
				t.Fatal(err)
			}
			expected["spec"].(map[string]interface{})["image"] = tc.image
			writes := 0
			setUpgradeTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if want := "/apis/rainbond.io/v1alpha1/namespaces/" + namespace + "/rbdcomponents/rbd-app-ui"; r.URL.Path != want {
					t.Errorf("unexpected request path: %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				switch r.Method {
				case http.MethodGet:
				case http.MethodPut, http.MethodPatch:
					writes++
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					if r.Method == http.MethodPatch {
						if got := r.Header.Get("Content-Type"); got != "application/merge-patch+json" {
							t.Errorf("unexpected patch type: %s", got)
						}
						stored, err = jsonpatch.MergePatch(stored, body)
						if err != nil {
							t.Error(err)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
					} else {
						stored = body
					}
				default:
					t.Errorf("unexpected method: %s", r.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				_, _ = w.Write(stored)
			})

			response := runUpgradeRequest(t, `{"rbd-app-ui":"`+tc.image+`"}`)
			if len(response.List.([]interface{})) != 0 {
				t.Fatalf("unexpected component errors: %v", response.List)
			}
			if writes != 1 {
				t.Fatalf("expected one component write, got %d", writes)
			}
			var actual map[string]interface{}
			if err := json.Unmarshal(stored, &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("upgrade must change only spec.image; got %s", stored)
			}
		})
	}
}

func TestUpgradeReportsComponentErrorsAndContinues(t *testing.T) {
	for _, tc := range []struct {
		name       string
		failMethod string
		message    string
	}{
		{name: "get failure", failMethod: http.MethodGet, message: "rbd-failed获取异常"},
		{name: "patch failure", failMethod: http.MethodPatch, message: "rbd-failed更新异常"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RBD_NAMESPACE", "rbd-system")
			upgraded := false
			setUpgradeTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				failed := strings.HasSuffix(r.URL.Path, "/rbd-failed")
				if failed && r.Method == tc.failMethod {
					w.WriteHeader(http.StatusForbidden)
					_, _ = io.WriteString(w, `{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"Forbidden","message":"access denied","code":403}`)
					return
				}
				name := "rbd-success"
				if failed {
					name = "rbd-failed"
				}
				if r.Method == http.MethodPatch && !failed {
					upgraded = true
				}
				_, _ = io.WriteString(w, `{"apiVersion":"rainbond.io/v1alpha1","kind":"RbdComponent","metadata":{"name":"`+name+`","namespace":"rbd-system"},"spec":{"image":"example.invalid/rainbond:old"}}`)
			})

			response := runUpgradeRequest(t, `{"rbd-failed":"example.invalid/rainbond:new","rbd-success":"example.invalid/rainbond:new"}`)
			failures, ok := response.List.([]interface{})
			if !ok || len(failures) != 1 {
				t.Fatalf("expected one component error, got %v", response.List)
			}
			if message, ok := failures[0].(string); !ok || !strings.Contains(message, tc.message) {
				t.Fatalf("expected error containing %q, got %v", tc.message, failures[0])
			}
			if !upgraded {
				t.Fatal("failure of one component must not prevent another component from upgrading")
			}
		})
	}
}

func runUpgradeRequest(t *testing.T, body string) httputil.ResponseBody {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v2/cluster/rbd-upgrade", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	(&ClusterController{}).Upgrade(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response httputil.ResponseBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func setUpgradeTestClient(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{v1alpha1.GroupVersion})
	mapper.Add(v1alpha1.GroupVersion.WithKind("RbdComponent"), meta.RESTScopeNamespace)
	client, err := k8sclient.New(&rest.Config{Host: server.URL}, k8sclient.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		t.Fatal(err)
	}
	component := k8s.Default()
	if component == nil {
		component = k8s.New()
	}
	previous := component.K8sClient
	component.K8sClient = client
	t.Cleanup(func() { component.K8sClient = previous })
}
