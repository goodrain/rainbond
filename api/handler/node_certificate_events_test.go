package handler

import (
	"context"
	"encoding/json"
	"github.com/goodrain/rainbond/api/model"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func nativeEvent(now time.Time, reason, message string) v1.Event {
	return v1.Event{ObjectMeta: metav1.ObjectMeta{Namespace: "default"}, InvolvedObject: v1.ObjectReference{Kind: "Node", Name: "worker", UID: "worker"}, Source: v1.EventSource{Component: "rke2-cert-monitor"}, Reason: reason, Message: message, LastTimestamp: metav1.NewTime(now)}
}

const warningMessage = "Node certificates require attention - restart rke2 on this node to trigger automatic rotation: kube-proxy/client-kube-proxy.crt: certificate CN=system:kube-proxy will expire within 120 days at 2026-11-18T09:00:00Z"

// capability_id: rainbond.nodes.native-certificate-events
func TestNativeCertificateEvents(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	warning := nativeEvent(now, "CertificateExpirationWarning", warningMessage)
	for _, tc := range []struct {
		name   string
		events []v1.Event
		status string
		count  int
	}{
		{"warning", []v1.Event{warning}, "ok", 1},
		{"expired and multiple", []v1.Event{nativeEvent(now, "CertificateExpirationWarning", warningMessage+", kubelet/client-kubelet.crt: certificate CN=kubelet expired at 2026-10-01T09:00:00Z")}, "ok", 2},
		{"new OK clears", []v1.Event{warning, nativeEvent(now.Add(time.Second), "CertificateExpirationOK", "Certificates checked OK")}, "ok", 0},
		{"equal OK clears", []v1.Event{warning, nativeEvent(now, "CertificateExpirationOK", "Certificates checked OK")}, "ok", 0},
		{"stale", []v1.Event{nativeEvent(now.Add(-66*time.Minute), warning.Reason, warning.Message)}, "unavailable", 0},
		{"future", []v1.Event{nativeEvent(now.Add(2*time.Minute), warning.Reason, warning.Message)}, "unavailable", 0},
		{"missing", nil, "unavailable", 0},
		{"malformed retains risk", []v1.Event{nativeEvent(now, warning.Reason, warning.Message+", kubelet/client-kubelet.crt: certificate CN=kubelet is not yet valid")}, "partial", 1},
		{"unknown server risk", []v1.Event{nativeEvent(now, warning.Reason, "server/client-controller.crt: certificate CN=controller expired at 2026-10-01T09:00:00Z")}, "unavailable", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodes := []model.NodeInfo{{Name: "worker", CertificateStatus: "unavailable"}}
			applyCertificateEvents(nodes, tc.events, now)
			if nodes[0].CertificateStatus != tc.status || len(nodes[0].Certificates) != tc.count {
				t.Fatal(nodes[0])
			}
			if tc.status != "unavailable" && (nodes[0].CertificateSource != "rke2_events" || nodes[0].CertificateObservedAt == "") {
				t.Fatal("missing provenance", nodes[0])
			}
		})
	}
	for _, mutate := range []func(*v1.Event){func(e *v1.Event) { e.Source.Component = "other" }, func(e *v1.Event) { e.InvolvedObject.Kind = "Pod" }, func(e *v1.Event) { e.InvolvedObject.Name = "deleted" }, func(e *v1.Event) { e.Reason = "Other" }} {
		event := warning
		mutate(&event)
		nodes := []model.NodeInfo{{Name: "worker", CertificateStatus: "unavailable"}}
		applyCertificateEvents(nodes, []v1.Event{event}, now)
		if nodes[0].CertificateStatus != "unavailable" {
			t.Fatal(nodes)
		}
	}
}
func TestNativeCertificateEventRequests(t *testing.T) {
	for _, mode := range []string{"ok", "failure", "timeout", "non-rke2"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/api/v1/namespaces/default/events" || r.URL.Query().Get("fieldSelector") != "involvedObject.kind=Node" {
					t.Error(r.URL.String())
				}
				if mode == "failure" {
					http.Error(w, "unavailable", 500)
					return
				}
				if mode == "timeout" {
					<-r.Context().Done()
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(v1.EventList{Items: []v1.Event{nativeEvent(time.Now(), "CertificateExpirationWarning", warningMessage)}})
			}))
			defer server.Close()
			client, _ := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
			nodes := []model.NodeInfo{{Name: "worker", CertificateStatus: "unavailable"}}
			if mode == "non-rke2" {
				nodes[0].CertificateStatus = "not_applicable"
			}
			start := time.Now()
			(&nodesHandle{clientset: client}).setNodesCertificates(context.Background(), nodes)
			if time.Since(start) > 2500*time.Millisecond {
				t.Fatal("deadline exceeded")
			}
			if mode == "ok" && nodes[0].CertificateStatus != "ok" {
				t.Fatal(nodes)
			}
			if mode == "non-rke2" && calls != 0 {
				t.Fatal("requested events for unsupported cluster")
			}
			if (mode == "failure" || mode == "timeout") && nodes[0].CertificateStatus != "unavailable" {
				t.Fatal(nodes)
			}
		})
	}
}

func TestNativeCertificateObservedTimeAndDuplicateRisks(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	event := nativeEvent(now.Add(-2*time.Hour), "CertificateExpirationWarning", warningMessage+"\n"+"kube-proxy/client-kube-proxy.crt: certificate CN=system:kube-proxy expired at 2026-10-01T09:00:00Z")
	event.Source.Component = ""
	event.ReportingController = "rke2-cert-monitor"
	event.Series = &v1.EventSeries{LastObservedTime: metav1.NewMicroTime(now)}
	nodes := []model.NodeInfo{{Name: "worker", CertificateStatus: "unavailable"}}
	applyCertificateEvents(nodes, []v1.Event{event}, now)
	if nodes[0].CertificateStatus != "ok" || len(nodes[0].Certificates) != 1 || nodes[0].Certificates[0].ExpiresAt != "2026-10-01T09:00:00Z" {
		t.Fatal(nodes)
	}
	event.Source.Component = "fake"
	nodes[0].CertificateStatus = "unavailable"
	nodes[0].Certificates = nil
	applyCertificateEvents(nodes, []v1.Event{event}, now)
	if len(nodes[0].Certificates) != 0 {
		t.Fatal("accepted conflicting event provenance")
	}
	for _, message := range []string{"garbage", warningMessage + ", kubelet/client-kubelet.crt: certificate CN=test expired at nonsense"} {
		certs, complete := parseCertificateWarning(message)
		if complete {
			t.Fatal("accepted malformed warning", certs)
		}
	}
}

func TestNativeCertificateEventsRejectReplacedNode(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, reason := range []string{"CertificateExpirationOK", "CertificateExpirationWarning"} {
		t.Run(reason, func(t *testing.T) {
			nodes := []model.NodeInfo{{Name: "worker", CreateTime: now, CertificateStatus: "unavailable"}}
			event := nativeEvent(now.Add(-time.Minute), reason, warningMessage)
			applyCertificateEvents(nodes, []v1.Event{event}, now)
			if nodes[0].CertificateStatus != "unavailable" || nodes[0].CertificateObservedAt != "" || len(nodes[0].Certificates) != 0 {
				t.Fatal("replaced node inherited previous node certificate check", nodes[0])
			}
		})
	}
}
