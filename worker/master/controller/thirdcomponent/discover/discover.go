// RAINBOND, Application Management Platform
// Copyright (C) 2021-2021 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.
// 该文件是 Rainbond 平台中的一个发现模块，用于监控和管理第三方组件的端点状态。文件实现了对不同来源的第三方组件进行发现和状态更新的功能。

// 文件的核心功能包括以下几部分：

// 1. `Discover` 接口：
//    - 定义了管理第三方组件端点的核心方法，如获取组件信息、执行发现操作、设置探测管理器等。
//    - 实现该接口的类可以用于不同类型的端点源，如 Kubernetes 服务或静态端点。

// 2. `NewDiscover` 函数：
//    - 根据给定的组件和配置信息创建并返回合适的 `Discover` 实例。
//    - 如果组件使用 Kubernetes 服务作为端点源，则返回 `kubernetesDiscover` 实例；
//      如果组件使用静态端点列表，则返回 `staticEndpoint` 实例。
//    - 如果不支持指定的端点源类型，则返回错误。

// 3. `kubernetesDiscover` 结构体：
//    - 实现了 `Discover` 接口，用于从 Kubernetes 服务中发现和更新端点信息。
//    - 包含组件信息和 Kubernetes 客户端，负责监控和更新组件的端点状态。

// 4. `kubernetesDiscover` 的主要方法：
//    - `GetComponent`：返回当前的组件信息。
//    - `getNamespace`：获取组件所在的命名空间，优先使用组件配置中的命名空间，否则使用组件本身的命名空间。
//    - `Discover`：持续监听 Kubernetes 服务的端点变化，当发生变化时，更新组件的端点状态并通知外部系统。
//    - `DiscoverOne`：立即获取并返回组件当前的端点状态，包括就绪和未就绪的端点。
//    - `SetProberManager`：设置探测管理器（目前未实现具体功能）。

// 5. `kubernetesDiscover` 的工作流程：
//    - 首先，通过 Kubernetes API 获取服务和端点的信息。
//    - 然后，使用 Kubernetes 的 Watch 机制持续监控端点的变化。
//    - 当端点发生变化时，调用 `DiscoverOne` 方法获取最新的端点状态，并更新组件的状态。
//    - 最后，将更新后的组件状态通过通道 `update` 发送出去，以便系统中的其他部分及时感知到变化。

// 该文件的实现确保了 Rainbond 平台能够准确、及时地发现和更新第三方组件的端点信息，无论这些端点是由 Kubernetes 服务提供，还是通过静态配置指定的。

package discover

import (
	"context"
	"fmt"
	"time"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	rainbondlistersv1alpha1 "github.com/goodrain/rainbond/pkg/generated/listers/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Discover -
type Discover interface {
	GetComponent() *v1alpha1.ThirdComponent
	DiscoverOne(ctx context.Context) ([]*v1alpha1.ThirdComponentEndpointStatus, error)
	Discover(ctx context.Context, update chan *v1alpha1.ThirdComponent) ([]*v1alpha1.ThirdComponentEndpointStatus, error)
	SetProberManager(proberManager prober.Manager)
}

// NewDiscover -
func NewDiscover(component *v1alpha1.ThirdComponent,
	restConfig *rest.Config,
	lister rainbondlistersv1alpha1.ThirdComponentLister) (Discover, error) {
	if component.Spec.EndpointSource.KubernetesService != nil {
		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			logrus.Errorf("create kube client error: %s", err.Error())
			return nil, err
		}
		return &kubernetesDiscover{
			component: component,
			client:    clientset,
		}, nil
	}
	if len(component.Spec.EndpointSource.StaticEndpoints) > 0 {
		return &staticEndpoint{
			component: component,
			lister:    lister,
		}, nil
	}
	return nil, fmt.Errorf("not support source type")
}

type kubernetesDiscover struct {
	component *v1alpha1.ThirdComponent
	client    *kubernetes.Clientset
}

func (k *kubernetesDiscover) GetComponent() *v1alpha1.ThirdComponent {
	return k.component
}
func (k *kubernetesDiscover) getNamespace() string {
	component := k.component
	namespace := component.Spec.EndpointSource.KubernetesService.Namespace
	if namespace == "" {
		namespace = component.Namespace
	}
	return namespace
}
func (k *kubernetesDiscover) Discover(ctx context.Context, update chan *v1alpha1.ThirdComponent) ([]*v1alpha1.ThirdComponentEndpointStatus, error) {
	namespace := k.getNamespace()
	component := k.component
	service, err := k.client.CoreV1().Services(namespace).Get(ctx, component.Spec.EndpointSource.KubernetesService.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("load kubernetes service failure %s", err.Error())
	}
	re, err := k.client.CoreV1().Endpoints(namespace).Watch(ctx, metav1.ListOptions{LabelSelector: labels.FormatLabels(service.Spec.Selector), Watch: true})
	if err != nil {
		return nil, fmt.Errorf("watch kubernetes endpoints failure %s", err.Error())
	}
	defer re.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, nil
		case <-re.ResultChan():
			func() {
				ctx, cancel := context.WithTimeout(ctx, time.Second*10)
				defer cancel()
				endpoints, err := k.DiscoverOne(ctx)
				if err == nil {
					new := component.DeepCopy()
					new.Status.Endpoints = endpoints
					update <- new
				} else {
					logrus.Errorf("discover kubernetes endpoints %s change failure %s", component.Spec.EndpointSource.KubernetesService.Name, err.Error())
				}
			}()
		}
	}
}
func (k *kubernetesDiscover) DiscoverOne(ctx context.Context) ([]*v1alpha1.ThirdComponentEndpointStatus, error) {
	component := k.component
	namespace := k.getNamespace()
	service, err := k.client.CoreV1().Services(namespace).Get(ctx, component.Spec.EndpointSource.KubernetesService.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("load kubernetes service failure %s", err.Error())
	}
	// service name must be same with endpoint name
	endpoint, err := k.client.CoreV1().Endpoints(namespace).Get(ctx, service.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("load kubernetes endpoints failure %s", err.Error())
	}
	getServicePort := func(portName string) int {
		for _, port := range service.Spec.Ports {
			if port.Name == portName {
				return int(port.Port)
			}
		}
		return 0
	}
	var es = []*v1alpha1.ThirdComponentEndpointStatus{}
	for _, subset := range endpoint.Subsets {
		for _, port := range subset.Ports {
			for _, address := range subset.Addresses {
				ed := v1alpha1.NewEndpointAddress(address.IP, int(port.Port))
				if ed != nil {
					es = append(es, &v1alpha1.ThirdComponentEndpointStatus{
						ServicePort: getServicePort(port.Name),
						Address:     *ed,
						TargetRef:   address.TargetRef,
						Status:      v1alpha1.EndpointReady,
					})
				}
			}
			for _, address := range subset.NotReadyAddresses {
				ed := v1alpha1.NewEndpointAddress(address.IP, int(port.Port))
				if ed != nil {
					es = append(es, &v1alpha1.ThirdComponentEndpointStatus{
						Address:     *ed,
						ServicePort: getServicePort(port.Name),
						TargetRef:   address.TargetRef,
						Status:      v1alpha1.EndpointReady,
					})
				}
			}
		}
	}
	return es, nil
}

func (k *kubernetesDiscover) SetProberManager(proberManager prober.Manager) {

}
