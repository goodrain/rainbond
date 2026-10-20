package handler

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/pkg/nodecert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// certificateCollectionCursor rotates node priority across requests, including new handler instances.
var certificateCollectionCursor atomic.Uint64

func validCertificateResult(result nodecert.Result, now time.Time) bool {
	collected, err := time.Parse(time.RFC3339, result.CollectedAt)
	if err != nil || now.Sub(collected) > 5*time.Minute || collected.Sub(now) > time.Minute {
		return false
	}
	count := len(result.Certificates)
	switch result.CertificateStatus {
	case "ok":
		if count != len(nodecert.RequiredFiles) {
			return false
		}
	case "partial":
		if count == 0 || count >= len(nodecert.RequiredFiles) {
			return false
		}
	case "unavailable":
		return count == 0
	default:
		return false
	}
	seen := map[string]bool{}
	for _, cert := range result.Certificates {
		allowed := false
		for _, name := range nodecert.RequiredFiles {
			if name == cert.Name {
				allowed = true
				break
			}
		}
		if !allowed || seen[cert.Name] {
			return false
		}
		seen[cert.Name] = true
		if _, err := time.Parse(time.RFC3339, cert.ExpiresAt); err != nil {
			return false
		}
	}
	return true
}

func (n *nodesHandle) setNodesCertificates(ctx context.Context, nodes []model.NodeInfo) {
	needsCollection := false
	for i := range nodes {
		if nodes[i].CertificateStatus == "unavailable" {
			needsCollection = true
			break
		}
	}
	if !needsCollection {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	pods, err := n.clientset.CoreV1().Pods(n.namespace).List(ctx, metav1.ListOptions{LabelSelector: "app=rbd-certificate-collector"})
	if err != nil {
		return
	}
	collectors := map[string]string{}
	for _, pod := range pods.Items {
		if pod.Spec.NodeName != "" && pod.DeletionTimestamp == nil && string(pod.Status.Phase) == "Running" && collectorReady(pod.Status.Conditions) {
			collectors[pod.Spec.NodeName] = pod.Name
		}
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := 8
	if len(nodes) < workers {
		workers = len(nodes)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				node := &nodes[index]
				pod := collectors[node.Name]
				if node.CertificateStatus != "unavailable" || pod == "" {
					continue
				}
				proxyCtx, cancelProxy := context.WithTimeout(ctx, 400*time.Millisecond)
				stream, err := n.clientset.CoreV1().Pods(n.namespace).ProxyGet("http", pod, "9351", "certificates", nil).Stream(proxyCtx)
				if err != nil {
					cancelProxy()
					continue
				}
				raw, err := io.ReadAll(io.LimitReader(stream, 64*1024+1))
				stream.Close()
				cancelProxy()
				if err != nil || len(raw) > 64*1024 {
					continue
				}
				var result nodecert.Result
				if json.Unmarshal(raw, &result) != nil || !validCertificateResult(result, time.Now()) {
					continue
				}
				node.CertificateStatus = result.CertificateStatus
				node.Certificates = result.Certificates
			}
		}()
	}
	start := int(certificateCollectionCursor.Add(uint64(workers))-uint64(workers)) % len(nodes)
queue:
	for offset := range nodes {
		i := (start + offset) % len(nodes)
		select {
		case jobs <- i:
		case <-ctx.Done():
			break queue
		}
	}
	close(jobs)
	wg.Wait()
}

func initialCertificateStatus(kubeletVersion string) string {
	if strings.Contains(strings.ToLower(kubeletVersion), "rke2") {
		return "unavailable"
	}
	return "not_applicable"
}

func collectorReady(conditions []v1.PodCondition) bool {
	for _, condition := range conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}
