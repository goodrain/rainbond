package conversion

import (
	"k8s.io/apimachinery/pkg/util/intstr"
	"time"

	"github.com/goodrain/rainbond/db"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/jinzhu/gorm"
	mv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//TenantServiceMonitor tenant service monitor
func TenantServiceMonitor(as *v1.AppService, dbmanager db.Manager) error {
	sms := createServiceMonitor(as, dbmanager)
	if sms != nil {
		for i := range sms {
			as.SetServiceMonitor(sms[i])
		}
	}
	return nil
}

func createServiceMonitor(as *v1.AppService, dbmanager db.Manager) []*mv1.ServiceMonitor {
	tsms, err := dbmanager.TenantServiceMonitorDao().GetByServiceID(as.ServiceID)
	if err != nil && err != gorm.ErrRecordNotFound {
		logrus.Errorf("get service %s monitor config failure %s", as.ServiceID, err.Error())
		return nil
	}
	if tsms == nil || len(tsms) == 0 {
		return nil
	}
	services := as.GetServices(false)
	var portService = make(map[int32]*corev1.Service, len(services))
	for i, s := range services {
		for _, port := range s.Spec.Ports {
			if _, exist := portService[port.Port]; exist {
				if s.Labels["service_type"] == "inner" {
					portService[port.Port] = services[i]
				}
			} else {
				portService[port.Port] = services[i]
			}
		}
	}
	var re []*mv1.ServiceMonitor
	for _, tsm := range tsms {
		if tsm.Name == "" {
			logrus.Warningf("service %s port %d service monitor name is empty", as.ServiceID, tsm.Port)
			continue
		}
		service, exist := portService[int32(tsm.Port)]
		if !exist {
			logrus.Warningf("service %s port %d not open, can not set monitor", as.ServiceID, tsm.Port)
			continue
		}
		// set service label app_name
		service.Labels["app_name"] = tsm.ServiceShowName
		as.SetService(service)
		if tsm.Path == "" {
			tsm.Path = "/metrics"
		}
		_, err = time.ParseDuration(tsm.Interval)
		if err != nil {
			logrus.Errorf("service monitor interval %s is valid, set default", tsm.Interval)
			tsm.Interval = "30s"
		}
		sm := mv1.ServiceMonitor{}
		sm.Name = tsm.Name
		sm.Labels = as.GetCommonLabels()
		sm.Namespace = as.GetNamespace()
		sm.Spec = mv1.ServiceMonitorSpec{
			// service label app_name
			JobLabel:          "app_name",
			NamespaceSelector: mv1.NamespaceSelector{Any: true},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"name": service.Labels["name"]},
			},
			Endpoints: []mv1.Endpoint{
				{
					TargetPort: &intstr.IntOrString{Type: intstr.Int, IntVal: int32(tsm.Port)},
					Path:       tsm.Path,
					Interval:   tsm.Interval,
				},
			},
		}
		re = append(re, &sm)
	}
	return re
}
