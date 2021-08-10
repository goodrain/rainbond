package discover

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	rainbondlistersv1alpha1 "github.com/goodrain/rainbond/pkg/generated/listers/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober/results"
)

type staticEndpoint struct {
	lister    rainbondlistersv1alpha1.ThirdComponentLister
	component *v1alpha1.ThirdComponent

	pmlock        sync.Mutex
	proberManager prober.Manager
}

func (s *staticEndpoint) GetComponent() *v1alpha1.ThirdComponent {
	return s.component
}

func (s *staticEndpoint) Discover(ctx context.Context, update chan *v1alpha1.ThirdComponent) ([]*v1alpha1.ThirdComponentEndpointStatus, error) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil, nil
		case <-ticker.C:
			// The method DiscoverOne of staticEndpoint does not need context.
			endpoints, _ := s.DiscoverOne(context.TODO())
			if !reflect.DeepEqual(endpoints, s.component.Status.Endpoints) {
				newComponent := s.component.DeepCopy()
				newComponent.Status.Endpoints = endpoints
				update <- newComponent
			}
		}
	}
}

func (s *staticEndpoint) DiscoverOne(ctx context.Context) ([]*v1alpha1.ThirdComponentEndpointStatus, error) {
	var res []*v1alpha1.ThirdComponentEndpointStatus
	for _, ep := range s.component.Spec.EndpointSource.StaticEndpoints {
		var addresses []*v1alpha1.EndpointAddress
		if ep.GetPort() != 0 {
			address := v1alpha1.NewEndpointAddress(ep.GetIP(), ep.GetPort())
			if address != nil {
				addresses = append(addresses, address)
			}
		} else {
			for _, port := range s.component.Spec.Ports {
				address := v1alpha1.NewEndpointAddress(ep.Address, port.Port)
				if address != nil {
					addresses = append(addresses, address)
				}
			}
		}
		if len(addresses) == 0 {
			continue
		}

		for _, address := range addresses {
			address := address
			es := &v1alpha1.ThirdComponentEndpointStatus{
				Address: *address,
				Status:  v1alpha1.EndpointReady,
			}
			res = append(res, es)

			// Make ready as the default status
			es.Status = v1alpha1.EndpointReady
			if s.proberManager != nil {
				result, found := s.proberManager.GetResult(s.component.GetEndpointID(es))
				if found && result != results.Success {
					es.Status = v1alpha1.EndpointNotReady
				}
			}
		}
	}

	return res, nil
}

func (s *staticEndpoint) SetProberManager(proberManager prober.Manager) {
	s.pmlock.Lock()
	defer s.pmlock.Unlock()
	s.proberManager = proberManager
}
