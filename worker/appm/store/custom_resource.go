// 该文件定义了一个管理自定义资源(Custom Resource, CR)的包，特别是与Prometheus监控相关的ServiceMonitor资源。
// 通过实现对Kubernetes自定义资源的获取和初始化，该包为Rainbond平台的应用运行时提供了监控和管理功能。

// 文件中的主要功能包括：
// 1. 自定义资源定义 (CRD) 的获取：通过 `GetCrds` 和 `GetCrd` 方法，提供了获取系统中所有CRD或特定CRD的功能。
//    这些功能允许平台在运行时动态地管理和访问Kubernetes中的自定义资源。
// 2. ServiceMonitor 客户端的管理：通过 `GetServiceMonitorClient` 方法，初始化并获取与ServiceMonitor相关的
//    Prometheus客户端，确保平台能够与Prometheus Operator进行交互，以实现应用服务的监控。
// 3. 自定义资源Informer的初始化：`initCustomResourceInformer` 方法用于初始化与ServiceMonitor相关的Informer，
//    并将其添加到事件处理程序中。通过定期同步和监听ServiceMonitor资源的变化，确保平台能够及时响应监控配置的更新。

// 总的来说，该文件通过定义和管理自定义资源，特别是ServiceMonitor，为Rainbond平台提供了与Prometheus监控集成的能力，
// 从而实现对应用服务的实时监控和管理。这种集成对于维护平台的稳定性和性能至关重要，特别是在大规模微服务架构中。

package store

import (
	"time"

	externalversions "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	"github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/sirupsen/logrus"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// ServiceMonitor service monitor custom resource
const ServiceMonitor = "servicemonitors.monitoring.coreos.com"

func (a *appRuntimeStore) GetCrds() (ret []*apiextensions.CustomResourceDefinition, err error) {
	return a.listers.CRD.List(nil)
}

func (a *appRuntimeStore) GetCrd(name string) (ret *apiextensions.CustomResourceDefinition, err error) {
	return a.listers.CRD.Get(name)
}

func (a *appRuntimeStore) GetServiceMonitorClient() (*versioned.Clientset, error) {
	if c := a.crClients["ServiceMonitor"]; c != nil {
		return c.(*versioned.Clientset), nil
	}
	c, err := versioned.NewForConfig(a.kubeconfig)
	if err != nil {
		return nil, err
	}
	a.crClients["ServiceMonitor"] = c
	return c, nil
}

func (a *appRuntimeStore) initCustomResourceInformer(stopch chan struct{}) {
	if cr, _ := a.GetCrd(ServiceMonitor); cr != nil {
		smc, err := a.GetServiceMonitorClient()
		if err != nil {
			logrus.Errorf("get service monitor client failure %s", err.Error())
		}
		if smc != nil {
			smFactory := externalversions.NewSharedInformerFactory(smc, 5*time.Minute)
			informer := smFactory.Monitoring().V1().ServiceMonitors().Informer()
			informer.AddEventHandlerWithResyncPeriod(a, time.Second*10)
			a.informers.CRS[ServiceMonitor] = informer
			logrus.Infof("[CRD] ServiceMonitor is inited")
		}
	}
	a.informers.StartCRS(stopch)
}
