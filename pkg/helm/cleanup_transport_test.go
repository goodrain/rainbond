package helm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"helm.sh/helm/v3/pkg/kube"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	"helm.sh/helm/v3/pkg/chartutil"
)

type noOpenAPITestClient struct{ kube.Interface }

func (c noOpenAPITestClient) Build(reader io.Reader, _ bool) (kube.ResourceList, error) {
	return c.Interface.Build(reader, false)
}

type helmTestManager struct {
	db.Manager
	database *gorm.DB
}

func (m helmTestManager) DB() *gorm.DB { return m.database }

func TestHelmTracksActualReleaseStorageWrites(t *testing.T) {
	for _, failure := range []string{"none", "workload", "release", "render-error"} {
		t.Run(failure, func(t *testing.T) { testHelmNativeWrites(t, failure) })
	}
}
func testHelmNativeWrites(t *testing.T, failure string) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "native.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := guard.RegisterStorage(database, guard.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/registry"}); err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", "store").Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	previous := db.GetManager()
	db.SetTestManager(helmTestManager{database: database})
	defer db.SetTestManager(previous)
	var writes, workloadWrites atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			docs := map[string]string{
				"/version":      `{"major":"1","minor":"30","gitVersion":"v1.30.0"}`,
				"/api":          `{"kind":"APIVersions","versions":["v1"]}`,
				"/apis":         `{"kind":"APIGroupList","groups":[{"name":"apps","versions":[{"groupVersion":"apps/v1","version":"v1"}],"preferredVersion":{"groupVersion":"apps/v1","version":"v1"}}]}`,
				"/api/v1":       `{"kind":"APIResourceList","groupVersion":"v1","resources":[{"name":"secrets","kind":"Secret","namespaced":true},{"name":"namespaces","kind":"Namespace","namespaced":false}]}`,
				"/apis/apps/v1": `{"kind":"APIResourceList","groupVersion":"apps/v1","resources":[{"name":"deployments","kind":"Deployment","namespaced":true}]}`,
			}
			if value, ok := docs[r.URL.Path]; ok {
				w.Write([]byte(value))
				return
			}
			if strings.HasSuffix(r.URL.Path, "/secrets") {
				w.Write([]byte(`{"apiVersion":"v1","kind":"SecretList","items":[]}`))
				return
			}
			w.WriteHeader(404)
			w.Write([]byte(`{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"NotFound","code":404}`))
			return
		}
		writes.Add(1)
		if strings.Contains(r.URL.Path, "/deployments") {
			workloadWrites.Add(1)
		}
		deletion := guard.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "delete", Owner: "test", Kind: "delete", Scope: "app", Target: "manifest", Fingerprint: "selected"}
		if _, err := guard.AcquireOperation(database, deletion); err != guard.ErrCoordinationBusy {
			t.Error("release write unprotected", err)
		}
		if (failure == "workload" && strings.Contains(r.URL.Path, "/deployments")) || (failure == "release" && strings.Contains(r.URL.Path, "/secrets")) {
			w.WriteHeader(500)
			w.Write([]byte(`{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"InternalError","code":500}`))
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()
	directory := t.TempDir()
	config := filepath.Join(directory, "kubeconfig")
	content := fmt.Sprintf("apiVersion: v1\nkind: Config\nclusters:\n- name: local-test\n  cluster:\n    server: %s\ncontexts:\n- name: local-test\n  context:\n    cluster: local-test\n    namespace: team\ncurrent-context: local-test\n", server.URL)
	if err := os.WriteFile(config, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", config)
	h, err := NewHelm("team", filepath.Join(directory, "repositories.yaml"), filepath.Join(directory, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	h.cfg.Capabilities = chartutil.DefaultCapabilities
	h.cfg.KubeClient = noOpenAPITestClient{Interface: h.cfg.KubeClient}
	chart := filepath.Join(directory, "chart")
	if err := os.MkdirAll(chart, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chart, "Chart.yaml"), []byte("apiVersion: v2\nname: fixture\nversion: 0.1.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(chart, "templates"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chart, "templates", "deployment.yaml"), []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: fixture\nspec:\n  selector:\n    matchLabels:\n      app: fixture\n  template:\n    metadata:\n      labels:\n        app: fixture\n    spec:\n      containers:\n      - name: app\n        image: fixture.invalid/app:v1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if failure == "render-error" {
		if err := os.WriteFile(filepath.Join(chart, "templates", "deployment.yaml"), []byte("{{ invalidFunction }}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, err = h.InstallFromChartPath(chart, "0.1.0", "fixture", "")
	if (failure == "none" && err != nil) || (failure != "none" && err == nil) {
		t.Fatal("unexpected native result", failure, err)
	}
	if failure == "render-error" && writes.Load() != 0 {
		t.Fatal("render error sent a mutation")
	}
	if failure == "none" && (writes.Load() < 3 || workloadWrites.Load() != 1) {
		t.Fatal("Helm release persistence was not exercised", writes.Load())
	}
	var operations []model.CleanupOperation
	expected := "finished"
	if failure == "workload" || failure == "release" {
		expected = "uncertain"
	}
	if err := database.Find(&operations).Error; err != nil || len(operations) != 1 || operations[0].State != expected {
		t.Fatal(operations, err)
	}
}

func TestHelmTransportKeepsUncertainWritesAndRejectsUnguardedWrites(t *testing.T) {
	tracker := &helmMutationTracker{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer server.Close()
	client := &http.Client{Transport: helmMutationTransport{next: http.DefaultTransport, tracker: tracker}}
	request, _ := http.NewRequest("POST", server.URL, bytes.NewReader(nil))
	if _, err := client.Do(request); err == nil {
		t.Fatal("unguarded write allowed")
	}
	state := tracker.begin()
	defer tracker.end(state)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if tracker.confirmed(state) {
		t.Fatal("500 treated as no effect")
	}
}
