package prober

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober/results"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/probe"
	httpprobe "k8s.io/kubernetes/pkg/probe/http"
	tcpprobe "k8s.io/kubernetes/pkg/probe/tcp"
)

const maxProbeRetries = 3

// Prober helps to check the readiness of a endpoint.
type prober struct {
	http httpprobe.Prober
	tcp  tcpprobe.Prober

	logger   *logrus.Entry
	recorder record.EventRecorder
}

// NewProber creates a Prober.
func newProber(
	recorder record.EventRecorder) *prober {
	return &prober{
		logger:   logrus.WithField("WHO", "Thirdcomponent Prober"),
		http:     httpprobe.New(true),
		tcp:      tcpprobe.New(),
		recorder: recorder,
	}
}

// probe probes the endpoint address.
func (pb *prober) probe(thirdComponent *v1alpha1.ThirdComponent, endpointStatus *v1alpha1.ThirdComponentEndpointStatus, endpointID string) (results.Result, error) {
	probeSpec := thirdComponent.Spec.Probe

	if probeSpec == nil {
		pb.logger.Warningf("probe for %s is nil", endpointID)
		return results.Success, nil
	}

	result, output, err := pb.runProbeWithRetries(probeSpec, thirdComponent, endpointStatus, endpointID, maxProbeRetries)
	if err != nil || (result != probe.Success) {
		// Probe failed in one way or another.
		if err != nil {
			pb.logger.Infof("probe for %q errored: %v", endpointID, err)
			pb.recordContainerEvent(thirdComponent, v1.EventTypeWarning, "EndpointUnhealthy", "probe errored: %v", err)
		} else { // result != probe.Success
			pb.logger.Debugf("probe for %q failed (%v): %s", endpointID, result, output)
			pb.recordContainerEvent(thirdComponent, v1.EventTypeWarning, "EndpointUnhealthy", "probe failed: %s", output)
		}
		return results.Failure, err
	}
	return results.Success, nil
}

// runProbeWithRetries tries to probe the container in a finite loop, it returns the last result
// if it never succeeds.
func (pb *prober) runProbeWithRetries(p *v1alpha1.Probe, thirdComponent *v1alpha1.ThirdComponent, endpointStatus *v1alpha1.ThirdComponentEndpointStatus, endpointID string, retries int) (probe.Result, string, error) {
	var err error
	var result probe.Result
	var output string
	for i := 0; i < retries; i++ {
		result, output, err = pb.runProbe(p, thirdComponent, endpointStatus, endpointID)
		if err == nil {
			return result, output, nil
		}
	}
	return result, output, err
}

func (pb *prober) runProbe(p *v1alpha1.Probe, thirdComponent *v1alpha1.ThirdComponent, endpointStatus *v1alpha1.ThirdComponentEndpointStatus, endpointID string) (probe.Result, string, error) {
	timeout := time.Duration(p.TimeoutSeconds) * time.Second

	if p.HTTPGet != nil {
		u, err := url.Parse(endpointStatus.Address.EnsureScheme())
		if err != nil {
			return probe.Unknown, "", err
		}
		headers := buildHeader(p.HTTPGet.HTTPHeaders)
		return pb.http.Probe(u, headers, timeout)
	}

	if p.TCPSocket != nil {
		return pb.tcp.Probe(endpointStatus.Address.GetIP(), endpointStatus.Address.GetPort(), timeout)
	}

	pb.logger.Warningf("Failed to find probe builder for endpoint address: %v", endpointID)
	return probe.Unknown, "", fmt.Errorf("missing probe handler for %s/%s", thirdComponent.Namespace, thirdComponent.Name)
}

// recordContainerEvent should be used by the prober for all endpoints related events.
func (pb *prober) recordContainerEvent(thirdComponent *v1alpha1.ThirdComponent, eventType, reason, message string, args ...interface{}) {
	pb.recorder.Eventf(thirdComponent, eventType, reason, message, args...)
}

// buildHeaderMap takes a list of HTTPHeader <name, value> string
// pairs and returns a populated string->[]string http.Header map.
func buildHeader(headerList []v1alpha1.HTTPHeader) http.Header {
	headers := make(http.Header)
	for _, header := range headerList {
		headers[header.Name] = append(headers[header.Name], header.Value)
	}
	return headers
}
