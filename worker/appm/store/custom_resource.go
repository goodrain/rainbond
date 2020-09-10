package store

import (
	"time"

	externalversions "github.com/coreos/prometheus-operator/pkg/client/informers/externalversions"
	"github.com/coreos/prometheus-operator/pkg/client/versioned"
	"github.com/sirupsen/logrus"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

//ServiceMonitor service monitor custom resource
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
