package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/pkg/nodecert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// capability_id: rainbond.nodes.certificate-payload
func TestValidateCertificatePayload(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	good := nodecert.Result{CollectedAt: now.Format(time.RFC3339), CertificateStatus: "ok", Certificates: []nodecert.Certificate{}}
	for _, name := range nodecert.RequiredFiles {
		good.Certificates = append(good.Certificates, nodecert.Certificate{Name: name, ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)})
	}
	if !validCertificateResult(good, now) {
		t.Fatal("rejected complete fresh payload")
	}
	for _, change := range []func(*nodecert.Result){func(r *nodecert.Result) { r.CollectedAt = now.Add(-6 * time.Minute).Format(time.RFC3339) }, func(r *nodecert.Result) { r.CertificateStatus = "healthy" }, func(r *nodecert.Result) { r.Certificates = r.Certificates[:1] }, func(r *nodecert.Result) { r.Certificates[0].Name = "private.key" }, func(r *nodecert.Result) { r.Certificates[0].ExpiresAt = "bad" }} {
		bad := good
		bad.Certificates = append([]nodecert.Certificate{}, good.Certificates...)
		change(&bad)
		if validCertificateResult(bad, now) {
			t.Fatal("accepted invalid collector payload", bad)
		}
	}
	good.CertificateStatus = "partial"
	good.Certificates = good.Certificates[:1]
	if !validCertificateResult(good, now) {
		t.Fatal("rejected partial metadata")
	}
}

func TestCertificateHealthPathAndUnavailable(t *testing.T) {
	for _, mode := range []string{"ok", "missing", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v1/nodes":
					w.Write([]byte(`{"apiVersion":"v1","kind":"NodeList","items":[{"metadata":{"name":"worker"},"status":{"nodeInfo":{"kubeletVersion":"v1.31.1+rke2r1"}}},{"metadata":{"name":"other"}}]}`))
				case "/api/v1/namespaces/rbd-system/pods":
					if r.URL.Query().Get("labelSelector") != "app=rbd-certificate-collector" {
						t.Error("missing label filter")
					}
					if mode == "missing" {
						w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[]}`))
						return
					}
					w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"collector"},"spec":{"nodeName":"worker"},"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"}]}}]}`))
				case "/api/v1/namespaces/rbd-system/pods/http:collector:9351/proxy/certificates":
					if mode == "timeout" {
						<-r.Context().Done()
						return
					}
					result := nodecert.Result{CollectedAt: time.Now().UTC().Format(time.RFC3339), CertificateStatus: "partial", Certificates: []nodecert.Certificate{{Name: nodecert.RequiredFiles[0], ExpiresAt: time.Now().UTC().Format(time.RFC3339)}}}
					json.NewEncoder(w).Encode(result)
				default:
					t.Error("unexpected request", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			handler := nodesHandle{namespace: "rbd-system", clientset: client}
			start := time.Now()
			nodes, err := handler.ListNodesWithOptions(context.Background(), false)
			if err != nil {
				t.Fatal(err)
			}
			if time.Since(start) > 2500*time.Millisecond {
				t.Fatal("certificate request exceeded deadline")
			}
			want := "unavailable"
			if mode == "ok" {
				want = "partial"
			}
			if nodes[0].CertificateStatus != want {
				t.Fatal(nodes)
			}
			if nodes[1].CertificateStatus != "not_applicable" {
				t.Fatal(nodes)
			}
		})
	}
}
func TestNoCollectorRequestForOtherKubernetes(t *testing.T) {
	nodes := []model.NodeInfo{{Name: "other", CertificateStatus: "not_applicable"}}
	(&nodesHandle{}).setNodesCertificates(context.Background(), nodes)
}

func TestCertificateCollectorsDoNotStarveLaterHealthyNode(t *testing.T) {
	nodes := make([]model.NodeInfo, 9)
	for i := range nodes {
		nodes[i] = model.NodeInfo{Name: fmt.Sprintf("node-%d", i), CertificateStatus: "unavailable"}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/namespaces/rbd-system/pods" {
			pods := v1.PodList{}
			for i := range nodes {
				pods.Items = append(pods.Items, v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("collector-%d", i)}, Spec: v1.PodSpec{NodeName: nodes[i].Name}, Status: v1.PodStatus{Phase: v1.PodRunning, Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionTrue}}}})
			}
			json.NewEncoder(w).Encode(pods)
			return
		}
		if !strings.Contains(r.URL.Path, "collector-8:") {
			<-r.Context().Done()
			return
		}
		json.NewEncoder(w).Encode(nodecert.Result{CollectedAt: time.Now().UTC().Format(time.RFC3339), CertificateStatus: "partial", Certificates: []nodecert.Certificate{{Name: nodecert.RequiredFiles[0], ExpiresAt: time.Now().UTC().Format(time.RFC3339)}}})
	}))
	defer server.Close()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	(&nodesHandle{namespace: "rbd-system", clientset: client}).setNodesCertificates(context.Background(), nodes)
	if nodes[8].CertificateStatus != "partial" {
		t.Fatal("healthy collector starved behind unavailable nodes")
	}
	if time.Since(start) > 2500*time.Millisecond {
		t.Fatal("overall budget exceeded")
	}
}
