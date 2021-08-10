package prober

import (
	"sync"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober/results"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/metrics"
)

// ProberResults stores the cumulative number of a probe by result as prometheus metrics.
var ProberResults = metrics.NewCounterVec(
	&metrics.CounterOpts{
		Subsystem:      "prober",
		Name:           "probe_total",
		Help:           "Cumulative number of a readiness probe for a thirdcomponent endpoint by result.",
		StabilityLevel: metrics.ALPHA,
	},
	[]string{"result",
		"endpoint",
		"thirdcomponent",
		"namespace"},
)

// Manager manages thirdcomponent probing. It creates a probe "worker" for every endpoint address that specifies a
// probe (AddThirdComponent). The worker periodically probes its assigned endpoint address and caches the results. The
// manager use the cached probe results to set the appropriate Ready state in the ThirdComponentEndpointStatus when
// requested (ThirdComponentEndpointStatus). Updating probe parameters is not currently supported.
type Manager interface {
	// AddThirdComponent creates new probe workers for every endpoint address probe.
	AddThirdComponent(thirdComponent *v1alpha1.ThirdComponent)

	// GetResult returns the probe result based on the given ID.
	GetResult(endpointID string) (results.Result, bool)

	Stop()

	// Updates creates a channel that receives an Update whenever its result changes (but not
	// removed).
	// NOTE: The current implementation only supports a single updates channel.
	Updates() <-chan results.Update
}

type manager struct {
	// Map of active workers for probes
	workers map[string]*worker
	// Lock for accessing & mutating workers
	workerLock sync.RWMutex

	// readinessManager manages the results of readiness probes
	readinessManager results.Manager

	// prober executes the probe actions.
	prober *prober

	// channel of updates
	updates chan results.Update
}

// NewManager creates a Manager for pod probing.
func NewManager(
	recorder record.EventRecorder) Manager {
	updates := make(chan results.Update)
	readinessManager := results.NewManager(updates)
	return &manager{
		prober:           newProber(recorder),
		readinessManager: readinessManager,
		workers:          make(map[string]*worker),
		updates:          updates,
	}
}

func (m *manager) AddThirdComponent(thirdComponent *v1alpha1.ThirdComponent) {
	if !thirdComponent.Spec.NeedProbe() {
		return
	}

	m.workerLock.Lock()
	defer m.workerLock.Unlock()

	newWorkers := make(map[string]*worker)
	for _, ep := range thirdComponent.Status.Endpoints {
		key := string(ep.Address)
		worker := newWorker(m, thirdComponent, *ep)
		oldWorker, ok := m.workers[key]
		if ok && worker.spec.Equals(oldWorker.spec) {
			newWorkers[key] = oldWorker
			delete(m.workers, key)
			continue
		}
		// run new worker
		newWorkers[key] = worker
		go worker.run()
	}

	// stop unused workers
	for _, worker := range m.workers {
		worker.stop()
	}

	m.workers = newWorkers
}

func (m *manager) Stop() {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()

	for _, worker := range m.workers {
		worker.stop()
	}
}

func (m *manager) GetResult(endpointID string) (results.Result, bool) {
	return m.readinessManager.Get(endpointID)
}

// Called by the worker after exiting.
func (m *manager) removeWorker(endpoint *v1alpha1.ThirdComponentEndpointStatus) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()
	delete(m.workers, string(endpoint.Address))
}

func (m *manager) Updates() <-chan results.Update {
	return m.updates
}
