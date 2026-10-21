package handler

import (
	"context"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/pkg/nodecert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const certificateEventSource = "rke2-cert-monitor"

var certificateRecord = regexp.MustCompile(`([A-Za-z0-9_./-]+\.crt): certificate `)
var certificateExpiry = regexp.MustCompile(`(?:will expire within [0-9]+ days at|expired at) ([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z)$`)

func (n *nodesHandle) setNodesCertificates(ctx context.Context, nodes []model.NodeInfo) {
	needed := false
	for i := range nodes {
		if nodes[i].CertificateStatus == "unavailable" {
			needed = true
			break
		}
	}
	if !needed {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	events, err := n.clientset.CoreV1().Events("default").List(ctx, metav1.ListOptions{FieldSelector: "involvedObject.kind=Node"})
	if err != nil {
		return
	}
	applyCertificateEvents(nodes, events.Items, time.Now())
}

func certificateEventTime(event v1.Event, now time.Time) time.Time {
	candidates := []time.Time{event.LastTimestamp.Time, event.EventTime.Time, event.FirstTimestamp.Time, event.CreationTimestamp.Time}
	if event.Series != nil {
		candidates = append(candidates, event.Series.LastObservedTime.Time)
	}
	var latest time.Time
	for _, candidate := range candidates {
		if candidate.After(now.Add(time.Minute)) {
			return time.Time{}
		}
		if candidate.After(latest) {
			latest = candidate
		}
	}
	if latest.IsZero() || now.Sub(latest) > 65*time.Minute {
		return time.Time{}
	}
	return latest
}

func applyCertificateEvents(nodes []model.NodeInfo, events []v1.Event, now time.Time) {
	type observation struct {
		event    v1.Event
		observed time.Time
	}
	latest := map[string]observation{}
	for _, event := range events {
		if event.InvolvedObject.Kind != "Node" || (event.Source.Component != certificateEventSource && event.ReportingController != certificateEventSource) || (event.Reason != "CertificateExpirationWarning" && event.Reason != "CertificateExpirationOK") {
			continue
		}
		// Conflicting provenance is not a native monitor event.
		if event.Source.Component != "" && event.Source.Component != certificateEventSource || event.ReportingController != "" && event.ReportingController != certificateEventSource {
			continue
		}
		observed := certificateEventTime(event, now)
		if observed.IsZero() {
			continue
		}
		previous, exists := latest[event.InvolvedObject.Name]
		if !exists || observed.After(previous.observed) || (observed.Equal(previous.observed) && event.Reason == "CertificateExpirationOK") {
			latest[event.InvolvedObject.Name] = observation{event, observed}
		}
	}
	for i := range nodes {
		node := &nodes[i]
		if node.CertificateStatus != "unavailable" {
			continue
		}
		observation, exists := latest[node.Name]
		if !exists || (!node.CreateTime.IsZero() && observation.observed.Before(node.CreateTime)) {
			continue
		}
		node.CertificateSource = "rke2_events"
		node.CertificateObservedAt = observation.observed.UTC().Format(time.RFC3339)
		node.Certificates = []nodecert.Certificate{}
		if observation.event.Reason == "CertificateExpirationOK" {
			node.CertificateStatus = "ok"
			continue
		}
		certificates, complete := parseCertificateWarning(observation.event.Message)
		node.Certificates = certificates
		if complete && len(certificates) > 0 {
			node.CertificateStatus = "ok"
		} else if len(certificates) > 0 {
			node.CertificateStatus = "partial"
		}
	}
}

func parseCertificateWarning(message string) ([]nodecert.Certificate, bool) {
	const prefix = "Node certificates require attention - restart rke2 on this node to trigger automatic rotation: "
	if !strings.HasPrefix(message, prefix) {
		return []nodecert.Certificate{}, false
	}
	message = strings.TrimPrefix(message, prefix)
	matches := certificateRecord.FindAllStringSubmatchIndex(message, -1)
	complete := len(matches) > 0 && matches[0][0] == 0
	expirations := map[string]time.Time{}
	for i, match := range matches {
		end := len(message)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		record := strings.TrimRight(message[match[1]:end], ", \r\n\t")
		name := ""
		basename := path.Base(message[match[2]:match[3]])
		for _, known := range nodecert.RequiredFiles {
			if path.Base(known) == basename {
				name = known
				break
			}
		}
		expiryMatch := certificateExpiry.FindStringSubmatch(record)
		if name == "" || len(expiryMatch) != 2 {
			complete = false
			continue
		}
		expiry, err := time.Parse(time.RFC3339, expiryMatch[1])
		if err != nil {
			complete = false
			continue
		}
		if previous, exists := expirations[name]; !exists || expiry.Before(previous) {
			expirations[name] = expiry
		}
	}
	certificates := make([]nodecert.Certificate, 0, len(expirations))
	for name, expiry := range expirations {
		certificates = append(certificates, nodecert.Certificate{Name: name, ExpiresAt: expiry.UTC().Format(time.RFC3339)})
	}
	sort.Slice(certificates, func(i, j int) bool { return certificates[i].Name < certificates[j].Name })
	return certificates, complete
}
func initialCertificateStatus(kubeletVersion string) string {
	if strings.Contains(strings.ToLower(kubeletVersion), "rke2") {
		return "unavailable"
	}
	return "not_applicable"
}
