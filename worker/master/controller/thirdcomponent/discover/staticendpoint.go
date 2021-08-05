package discover

import (
	"context"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	rainbondlistersv1alpha1 "github.com/goodrain/rainbond/pkg/generated/listers/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober/results"
	"github.com/pkg/errors"
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
	endpointStatues, err := s.DiscoverOne(ctx)
	if err != nil {
		return nil, err
	}
	return endpointStatues, nil
}

func (s *staticEndpoint) DiscoverOne(ctx context.Context) ([]*v1alpha1.ThirdComponentEndpointStatus, error) {
	// Optimization: reduce the press of database if necessary.
	endpoints, err := db.GetManager().EndpointsDao().ListIsOnline(s.component.GetComponentID())
	if err != nil {
		return nil, errors.WithMessage(err, "list online static endpoints")
	}

	var res []*v1alpha1.ThirdComponentEndpointStatus
	for _, ep := range endpoints {
		ed := v1alpha1.NewEndpointAddress(ep.IP, ep.Port)
		if ed == nil {
			continue
		}

		es := &v1alpha1.ThirdComponentEndpointStatus{
			ServicePort: ep.Port,
			Address:     v1alpha1.EndpointAddress(ep.GetAddress()),
			Status:      v1alpha1.EndpointReady,
		}
		res = append(res, es)

		result, found := s.proberManager.GetResult(s.component.GetEndpointID(es))
		if found || result != results.Success {
			es.Status = v1alpha1.EndpointNotReady
		}
		// not found means no need probe, set status to ready as default
		es.Status = v1alpha1.EndpointReady
	}

	return res, nil
}
