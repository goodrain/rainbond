package discover

import (
	"context"
	"reflect"
	"sync"

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
	for {
		select {
		case <-ctx.Done():
			return nil, nil
		case <-s.proberManager.Updates():
			s.discoverOne(update)
		}
	}
}

func (s *staticEndpoint) DiscoverOne(ctx context.Context) ([]*v1alpha1.ThirdComponentEndpointStatus, error) {
	component := s.component
	var endpoints []*v1alpha1.ThirdComponentEndpointStatus
	for _, ep := range component.Spec.EndpointSource.StaticEndpoints {
		if ep.GetPort() != 0 {
			address := v1alpha1.NewEndpointAddress(ep.GetIP(), ep.GetPort())
			if address != nil {
				endpoints = append(endpoints, &v1alpha1.ThirdComponentEndpointStatus{
					Address: *address,
					Name:    ep.Name,
				})
			}
		} else {
			for _, port := range component.Spec.Ports {
				address := v1alpha1.NewEndpointAddress(ep.Address, port.Port)
				if address != nil {
					endpoints = append(endpoints, &v1alpha1.ThirdComponentEndpointStatus{
						Address: *address,
						Name:    ep.Name,
					})
				}
			}
		}

		for _, ep := range endpoints {
			// Make ready as the default status
			ep.Status = v1alpha1.EndpointReady
		}
	}

	// Update status with probe result
	if s.proberManager != nil {
		var newEndpoints []*v1alpha1.ThirdComponentEndpointStatus
		for _, ep := range endpoints {
			result, found := s.proberManager.GetResult(s.component.GetEndpointID(ep))
			if !found {
				// NotReady means the endpoint should not be online.
				ep.Status = v1alpha1.EndpointNotReady
			}
			if result != results.Success {
				ep.Status = v1alpha1.EndpointUnhealthy
			}
			newEndpoints = append(newEndpoints, ep)
		}
		return newEndpoints, nil
	}

	return endpoints, nil
}

func (s *staticEndpoint) SetProberManager(proberManager prober.Manager) {
	s.pmlock.Lock()
	defer s.pmlock.Unlock()
	s.proberManager = proberManager
}

func (s *staticEndpoint) discoverOne(update chan *v1alpha1.ThirdComponent) {
	component := s.component
	// The method DiscoverOne of staticEndpoint does not need context.
	endpoints, _ := s.DiscoverOne(context.TODO())
	if !reflect.DeepEqual(endpoints, component.Status.Endpoints) {
		newComponent := s.component.DeepCopy()
		newComponent.Status.Endpoints = endpoints
		update <- newComponent
	}
}
