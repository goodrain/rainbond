// 该文件是 Rainbond 平台中用于发现和管理静态端点的模块，实现了对第三方组件的静态端点进行探测和状态更新的功能。

// 文件的主要功能包括以下几个部分：

// 1. `staticEndpoint` 结构体：
//    - 该结构体实现了 `Discover` 接口，主要用于处理静态配置的端点源。
//    - 通过 `ThirdComponentLister` 列表器获取第三方组件的列表，并管理与这些组件相关的探测任务。
//    - 包含一个 `proberManager`，用于管理端点的探测任务和结果。

// 2. `GetComponent` 方法：
//    - 返回当前的第三方组件信息。

// 3. `Discover` 方法：
//    - 该方法持续监听上下文或探测管理器的更新事件。
//    - 当探测管理器有更新时，调用 `discoverOne` 方法探测端点状态，并将更新的组件信息通过通道 `update` 发送出去。

// 4. `DiscoverOne` 方法：
//    - 该方法根据组件的静态配置生成端点状态列表，并进行初步的状态设置（如设置为 "就绪" 状态）。
//    - 如果配置了 `proberManager`，会进一步根据探测结果更新端点的状态，如将不健康的端点设置为 "不健康" 状态。

// 5. `SetProberManager` 方法：
//    - 用于设置探测管理器 `proberManager`，管理与该组件相关的探测任务和结果。

// 6. `discoverOne` 方法：
//    - 该方法不依赖上下文，直接调用 `DiscoverOne` 方法获取当前组件的端点状态，并将其与之前的状态进行对比。
//    - 如果端点状态发生变化，则创建一个新的组件对象并将其更新状态发送到 `update` 通道。

// 通过上述功能，`staticEndpoint` 实现了对静态端点的管理，能够定期探测端点状态，并根据探测结果更新组件的状态，确保系统中的第三方组件能够保持正确的运行状态。

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
				address := v1alpha1.NewEndpointAddress(ep.GetIP(), port.Port)
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
