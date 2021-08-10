package prober

import (
	"math/rand"
	"time"

	"github.com/docker/go-metrics"
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober/results"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/runtime"
)

const (
	probeResultSuccessful string = "successful"
	probeResultFailed     string = "failed"
	probeResultUnknown    string = "unknown"
)

// worker handles the periodic probing of its assigned container. Each worker has a go-routine
// associated with it which runs the probe loop until the container permanently terminates, or the
// stop channel is closed. The worker uses the probe Manager's statusManager to get up-to-date
// container IDs.
type worker struct {
	// Channel for stopping the probe.
	stopCh chan struct{}

	// The pod containing this probe (read-only)
	thirdComponent *v1alpha1.ThirdComponent

	// The endpoint to probe (read-only)
	endpoint v1alpha1.ThirdComponentEndpointStatus

	// Describes the probe configuration (read-only)
	spec *v1alpha1.Probe

	// The probe value during the initial delay.
	initialValue results.Result

	// Where to store this workers results.
	resultsManager results.Manager
	probeManager   *manager

	// The last probe result for this worker.
	lastResult results.Result
	// How many times in a row the probe has returned the same result.
	resultRun int

	// proberResultsMetricLabels holds the labels attached to this worker
	// for the ProberResults metric by result.
	proberResultsSuccessfulMetricLabels metrics.Labels
	proberResultsFailedMetricLabels     metrics.Labels
	proberResultsUnknownMetricLabels    metrics.Labels
}

// Creates and starts a new probe worker.
func newWorker(
	m *manager,
	thirdComponent *v1alpha1.ThirdComponent,
	endpoint v1alpha1.ThirdComponentEndpointStatus) *worker {

	w := &worker{
		stopCh:         make(chan struct{}, 1), // Buffer so stop() can be non-blocking.
		probeManager:   m,
		thirdComponent: thirdComponent,
		endpoint:       endpoint,
	}

	w.spec = thirdComponent.Spec.Probe
	w.resultsManager = m.readinessManager
	w.initialValue = results.Failure

	basicMetricLabels := metrics.Labels{
		"endpoint":  string(w.endpoint.Address),
		"pod":       w.thirdComponent.Name,
		"namespace": w.thirdComponent.Namespace,
	}

	w.proberResultsSuccessfulMetricLabels = deepCopyPrometheusLabels(basicMetricLabels)
	w.proberResultsSuccessfulMetricLabels["result"] = probeResultSuccessful

	w.proberResultsFailedMetricLabels = deepCopyPrometheusLabels(basicMetricLabels)
	w.proberResultsFailedMetricLabels["result"] = probeResultFailed

	w.proberResultsUnknownMetricLabels = deepCopyPrometheusLabels(basicMetricLabels)
	w.proberResultsUnknownMetricLabels["result"] = probeResultUnknown

	return w
}

// run periodically probes the endpoint.
func (w *worker) run() {
	logrus.Infof("start prober worker %s", w.thirdComponent.GetEndpointID(&w.endpoint))

	probeTickerPeriod := time.Duration(w.spec.PeriodSeconds) * time.Second

	// If kubelet restarted the probes could be started in rapid succession.
	// Let the worker wait for a random portion of tickerPeriod before probing.
	time.Sleep(time.Duration(rand.Float64() * float64(probeTickerPeriod)))

	probeTicker := time.NewTicker(probeTickerPeriod)

	defer func() {
		// Clean up.
		probeTicker.Stop()
		w.resultsManager.Remove(w.thirdComponent.GetEndpointID(&w.endpoint))

		w.probeManager.removeWorker(&w.endpoint)
		ProberResults.Delete(w.proberResultsSuccessfulMetricLabels)
		ProberResults.Delete(w.proberResultsFailedMetricLabels)
		ProberResults.Delete(w.proberResultsUnknownMetricLabels)
	}()

probeLoop:
	for w.doProbe() {
		// Wait for next probe tick.
		select {
		case <-w.stopCh:
			break probeLoop
		case <-probeTicker.C:
			// continue
		}
	}
}

// stop stops the probe worker. The worker handles cleanup and removes itself from its manager.
// It is safe to call stop multiple times.
func (w *worker) stop() {
	select {
	case w.stopCh <- struct{}{}:
	default: // Non-blocking.
	}
}

// doProbe probes the endpint once and records the result.
// Returns whether the worker should continue.
func (w *worker) doProbe() (keepGoing bool) {
	defer func() { recover() }() // Actually eat panics (HandleCrash takes care of logging)
	defer runtime.HandleCrash(func(_ interface{}) { keepGoing = true })

	result, err := w.probeManager.prober.probe(w.thirdComponent, &w.endpoint, w.thirdComponent.GetEndpointID(&w.endpoint))
	if err != nil {
		// Prober error, throw away the result.
		return true
	}

	switch result {
	case results.Success:
		ProberResults.With(w.proberResultsSuccessfulMetricLabels).Inc()
	case results.Failure:
		ProberResults.With(w.proberResultsFailedMetricLabels).Inc()
	default:
		ProberResults.With(w.proberResultsUnknownMetricLabels).Inc()
	}

	if w.lastResult == result {
		w.resultRun++
	} else {
		w.lastResult = result
		w.resultRun = 1
	}

	if (result == results.Failure && w.resultRun < int(w.spec.FailureThreshold)) ||
		(result == results.Success && w.resultRun < int(w.spec.SuccessThreshold)) {
		// Success or failure is below threshold - leave the probe state unchanged.
		return true
	}

	w.resultsManager.Set(w.thirdComponent.GetEndpointID(&w.endpoint), result)

	return true
}

func deepCopyPrometheusLabels(m metrics.Labels) metrics.Labels {
	ret := make(metrics.Labels, len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}
