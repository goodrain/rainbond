package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/goodrain/rainbond/api/model"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
		{"stale", []v1.Event{nativeEvent(now.Add(-66*time.Minute), warning.Reason, warning.Message)}, "not_observed", 0},
		{"stale OK", []v1.Event{nativeEvent(now.Add(-66*time.Minute), "CertificateExpirationOK", "Certificates checked OK")}, "not_observed", 0},
		{"future", []v1.Event{nativeEvent(now.Add(2*time.Minute), warning.Reason, warning.Message)}, "not_observed", 0},
		{"missing", nil, "not_observed", 0},
		{"malformed retains risk", []v1.Event{nativeEvent(now, warning.Reason, warning.Message+", kubelet/client-kubelet.crt: certificate CN=kubelet is not yet valid")}, "partial", 1},
		{"unknown server risk", []v1.Event{nativeEvent(now, warning.Reason, "server/client-controller.crt: certificate CN=controller expired at 2026-10-01T09:00:00Z")}, "unavailable", 0},
		{"unknown certificate risk", []v1.Event{nativeEvent(now, warning.Reason, "Node certificates require attention - restart rke2 on this node to trigger automatic rotation: server/client-controller.crt: certificate CN=controller expired at 2026-10-01T09:00:00Z")}, "unavailable", 0},
		{"CA risk", []v1.Event{nativeEvent(now, "CACertificateExpirationWarning", "Certificate authority certificates require attention")}, "unavailable", 0},
		{"CA risk survives newer leaf OK", []v1.Event{nativeEvent(now.Add(-time.Minute), "CACertificateExpirationWarning", "Certificate authority certificates require attention"), nativeEvent(now, "CertificateExpirationOK", "Node certificates OK")}, "unavailable", 0},
		{"CA risk survives equal leaf OK", []v1.Event{nativeEvent(now, "CACertificateExpirationWarning", "Certificate authority certificates require attention"), nativeEvent(now, "CertificateExpirationOK", "Node certificates OK")}, "unavailable", 0},
		{"CA risk survives equal leaf OK reversed", []v1.Event{nativeEvent(now, "CertificateExpirationOK", "Node certificates OK"), nativeEvent(now, "CACertificateExpirationWarning", "Certificate authority certificates require attention")}, "unavailable", 0},
		{"CA and leaf risk both retained", []v1.Event{nativeEvent(now.Add(-time.Minute), "CACertificateExpirationWarning", "Certificate authority certificates require attention"), warning}, "partial", 1},
		{"stale CA does not block leaf OK", []v1.Event{nativeEvent(now.Add(-66*time.Minute), "CACertificateExpirationWarning", "Certificate authority certificates require attention"), nativeEvent(now, "CertificateExpirationOK", "Node certificates OK")}, "ok", 0},
		{"future CA does not block leaf OK", []v1.Event{nativeEvent(now.Add(2*time.Minute), "CACertificateExpirationWarning", "Certificate authority certificates require attention"), nativeEvent(now, "CertificateExpirationOK", "Node certificates OK")}, "ok", 0},
		{"malformed native risk", []v1.Event{nativeEvent(now, warning.Reason, "unrecognized warning")}, "unavailable", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodes := []model.NodeInfo{{Name: "worker", CertificateStatus: "unavailable"}}
			applyCertificateEvents(nodes, tc.events, now)
			if nodes[0].CertificateStatus != tc.status || len(nodes[0].Certificates) != tc.count {
				t.Fatal(nodes[0])
			}
			if nodes[0].CertificateSource != "rke2_events" || nodes[0].CertificateCheckedAt != now.Format(time.RFC3339) {
				t.Fatal("missing provenance", nodes[0])
			}
			if (tc.status == "not_observed") != (nodes[0].CertificateObservedAt == "") {
				t.Fatal("event observation timestamp does not match actual event coverage", nodes[0])
			}
			if tc.name == "CA risk survives newer leaf OK" && nodes[0].CertificateObservedAt != now.Format(time.RFC3339) {
				t.Fatal("leaf observation did not retain its own timestamp", nodes[0])
			}
		})
	}
	for _, mutate := range []func(*v1.Event){func(e *v1.Event) { e.Source.Component = "other" }, func(e *v1.Event) { e.InvolvedObject.Kind = "Pod" }, func(e *v1.Event) { e.InvolvedObject.Name = "deleted" }, func(e *v1.Event) { e.Reason = "Other" }} {
		event := warning
		mutate(&event)
		nodes := []model.NodeInfo{{Name: "worker", CertificateStatus: "unavailable"}}
		applyCertificateEvents(nodes, []v1.Event{event}, now)
		if nodes[0].CertificateStatus != "not_observed" || nodes[0].CertificateObservedAt != "" {
			t.Fatal(nodes)
		}
	}
	for _, status := range []string{"ok", "partial", "not_applicable"} {
		t.Run("preserve "+status, func(t *testing.T) {
			before := model.NodeInfo{Name: "worker", CertificateStatus: status, CertificateSource: "existing", CertificateObservedAt: "existing observation", CertificateCheckedAt: "existing check"}
			nodes := []model.NodeInfo{before}
			applyCertificateEvents(nodes, nil, now)
			if !reflect.DeepEqual(nodes[0], before) {
				t.Fatal("overwrote an ineligible certificate result", nodes[0])
			}
		})
	}
	if initialCertificateStatus("v1.30.4+rke2r1") != "unavailable" || initialCertificateStatus("v1.30.4") != "not_applicable" {
		t.Fatal("certificate observation must only apply to RKE2 nodes")
	}
}
func TestNativeCertificateEventRequests(t *testing.T) {
	for _, mode := range []string{"ok", "empty", "ordinary-event", "failure", "timeout", "non-rke2"} {
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
				events := []v1.Event{nativeEvent(time.Now(), "CertificateExpirationWarning", warningMessage)}
				if mode == "empty" {
					events = nil
				}
				if mode == "ordinary-event" {
					events[0].Reason = "NodeReady"
					events[0].Source.Component = "kubelet"
				}
				json.NewEncoder(w).Encode(v1.EventList{Items: events})
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
			if mode == "empty" || mode == "ordinary-event" {
				if nodes[0].CertificateStatus != "not_observed" || nodes[0].CertificateSource != "rke2_events" || nodes[0].CertificateObservedAt != "" || len(nodes[0].Certificates) != 0 {
					t.Fatal("successful event query fabricated certificate coverage or looked unavailable", nodes[0])
				}
			}
			encoded, err := json.Marshal(nodes[0])
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatal(err)
			}
			checkedAt, _ := payload["certificate_checked_at"].(string)
			if mode == "ok" || mode == "empty" || mode == "ordinary-event" {
				checked, err := time.Parse(time.RFC3339, checkedAt)
				if err != nil || checked.Before(start.Truncate(time.Second)) || checked.After(time.Now()) {
					t.Fatal("successful event query missing its actual check timestamp", checkedAt)
				}
			} else if checkedAt != "" || nodes[0].CertificateSource != "" || nodes[0].CertificateObservedAt != "" {
				t.Fatal("failed or unsupported query fabricated observation metadata", nodes[0])
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
	for _, reason := range []string{"CertificateExpirationOK", "CertificateExpirationWarning", "CACertificateExpirationWarning"} {
		t.Run(reason, func(t *testing.T) {
			nodes := []model.NodeInfo{{Name: "worker", CreateTime: now, CertificateStatus: "unavailable"}}
			event := nativeEvent(now.Add(-time.Minute), reason, warningMessage)
			applyCertificateEvents(nodes, []v1.Event{event}, now)
			if nodes[0].CertificateStatus != "not_observed" || nodes[0].CertificateObservedAt != "" || len(nodes[0].Certificates) != 0 {
				t.Fatal("replaced node inherited previous node certificate check", nodes[0])
			}
		})
	}
}

func TestNativeCertificateWarningsPreserveUnparsedRisks(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, tc := range []struct {
		name   string
		events []v1.Event
		want   []map[string]interface{}
	}{
		{"CA warning", []v1.Event{nativeEvent(now, "CACertificateExpirationWarning", "Certificate authority certificates require attention")}, []map[string]interface{}{{"reason": "CACertificateExpirationWarning", "observed_at": now.Format(time.RFC3339)}}},
		{"unparsed leaf warning", []v1.Event{nativeEvent(now, "CertificateExpirationWarning", "unrecognized warning")}, []map[string]interface{}{{"reason": "CertificateExpirationWarning", "observed_at": now.Format(time.RFC3339)}}},
		{"partial leaf warning", []v1.Event{nativeEvent(now, "CertificateExpirationWarning", warningMessage+", server/unknown.crt: certificate CN=other expired at 2026-10-01T09:00:00Z")}, []map[string]interface{}{{"reason": "CertificateExpirationWarning", "observed_at": now.Format(time.RFC3339)}}},
		{"leaf OK does not refresh CA warning", []v1.Event{nativeEvent(now.Add(-time.Minute), "CACertificateExpirationWarning", "Certificate authority certificates require attention"), nativeEvent(now, "CertificateExpirationOK", "Certificates checked OK")}, []map[string]interface{}{{"reason": "CACertificateExpirationWarning", "observed_at": now.Add(-time.Minute).Format(time.RFC3339)}}},
		{"CA and unparsed leaf warnings stay separate", []v1.Event{nativeEvent(now, "CertificateExpirationWarning", "unrecognized warning"), nativeEvent(now.Add(-time.Minute), "CACertificateExpirationWarning", "Certificate authority certificates require attention")}, []map[string]interface{}{{"reason": "CACertificateExpirationWarning", "observed_at": now.Add(-time.Minute).Format(time.RFC3339)}, {"reason": "CertificateExpirationWarning", "observed_at": now.Format(time.RFC3339)}}},
		{"known leaf warning has exact expiration", []v1.Event{nativeEvent(now, "CertificateExpirationWarning", warningMessage)}, nil},
		{"leaf OK clears leaf warning", []v1.Event{nativeEvent(now.Add(-time.Minute), "CertificateExpirationWarning", "unrecognized warning"), nativeEvent(now, "CertificateExpirationOK", "Certificates checked OK")}, nil},
		{"stale warnings are not current risks", []v1.Event{nativeEvent(now.Add(-66*time.Minute), "CACertificateExpirationWarning", "Certificate authority certificates require attention")}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodes := []model.NodeInfo{{Name: "worker", CertificateStatus: "unavailable"}}
			applyCertificateEvents(nodes, tc.events, now)
			encoded, err := json.Marshal(nodes[0])
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatal(err)
			}
			var warnings []map[string]interface{}
			if raw, ok := payload["certificate_warnings"]; ok {
				if err := json.Unmarshal(raw, &warnings); err != nil {
					t.Fatal(err)
				}
			}
			if len(warnings) != len(tc.want) || len(warnings) > 0 && !reflect.DeepEqual(warnings, tc.want) {
				t.Fatalf("warnings = %#v, want %#v", warnings, tc.want)
			}
		})
	}
}

func TestNativeCertificateLeafObservationDoesNotInheritCAAge(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	nodes := []model.NodeInfo{{Name: "worker", CertificateStatus: "unavailable"}}
	applyCertificateEvents(nodes, []v1.Event{
		nativeEvent(now.Add(-65*time.Minute), "CACertificateExpirationWarning", "Certificate authority certificates require attention"),
		nativeEvent(now, "CertificateExpirationWarning", warningMessage),
	}, now)
	if nodes[0].CertificateObservedAt != now.Format(time.RFC3339) || len(nodes[0].Certificates) != 1 {
		t.Fatal("CA observation age must not expire a current leaf result", nodes[0])
	}
	if len(nodes[0].CertificateWarnings) != 1 || nodes[0].CertificateWarnings[0].ObservedAt != now.Add(-65*time.Minute).Format(time.RFC3339) {
		t.Fatal("leaf result must not refresh a separate CA risk", nodes[0])
	}
}
