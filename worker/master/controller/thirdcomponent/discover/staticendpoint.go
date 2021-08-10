package discover

import (
	"context"
	"reflect"
	"time"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	rainbondlistersv1alpha1 "github.com/goodrain/rainbond/pkg/generated/listers/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober/results"
)

type staticEndpoint struct {
	lister        rainbondlistersv1alpha1.ThirdComponentLister
	component     *v1alpha1.ThirdComponent
	proberManager prober.Manager
}

func (s *staticEndpoint) GetComponent() *v1alpha1.ThirdComponent {
	return s.component
}

func (s *staticEndpoint) Discover(ctx context.Context, update chan *v1alpha1.ThirdComponent) ([]*v1alpha1.ThirdComponentEndpointStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	endpoints, err := s.DiscoverOne(ctx)
	if err != nil {
		return nil, err
	}

	newComponent := s.component.DeepCopy()
	newComponent.Status.Endpoints = endpoints
	if !reflect.DeepEqual(endpoints, s.component.Status.Endpoints) {
		update <- newComponent
	}

	return endpoints, nil
}

func (s *staticEndpoint) DiscoverOne(ctx context.Context) ([]*v1alpha1.ThirdComponentEndpointStatus, error) {
	var res []*v1alpha1.ThirdComponentEndpointStatus
	for _, ep := range s.component.Spec.EndpointSource.StaticEndpoints {
		var addresses []*v1alpha1.EndpointAddress
		if ep.GetPort() != 0 {
			addresses = append(addresses, v1alpha1.NewEndpointAddress(ep.Address, ep.GetPort()))
		} else {
			for _, port := range s.component.Spec.Ports {
				addresses = append(addresses, v1alpha1.NewEndpointAddress(ep.Address, port.Port))
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

			result, found := s.proberManager.GetResult(s.component.GetEndpointID(es))
			es.Status = v1alpha1.EndpointReady
			if found && result != results.Success {
				es.Status = v1alpha1.EndpointNotReady
			}
		}
	}

	return res, nil
}
