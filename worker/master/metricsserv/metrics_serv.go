// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package metricsserv

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/pquerna/ffjson/ffjson"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	kubeaggregatorclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// MetricsServiceManager -
type MetricsServiceManager struct {
	clientset                kubernetes.Interface
	apiregistrationClientset kubeaggregatorclientset.Interface
	clientv3                 *clientv3.Client

	stopCh chan struct{}
}

type metricsServerEndpoint struct {
	Address string
	Port    int
}

//New new
func New(clientset kubernetes.Interface, apiregistrationClientset kubeaggregatorclientset.Interface, clientv3 *clientv3.Client) *MetricsServiceManager {
	msm := &MetricsServiceManager{
		clientset:                clientset,
		apiregistrationClientset: apiregistrationClientset,
		clientv3:                 clientv3,
	}

	return msm
}

//Start start
func (m *MetricsServiceManager) Start() error {
	if err := m.newMetricsServerAPIService(); err != nil {
		return err
	}

	if err := m.newMetricsServerService(); err != nil {
		return err
	}

	if err := m.newMetricsServiceEndpoints(); err != nil {
		return err
	}

	return nil
}

func (m *MetricsServiceManager) newMetricsServerAPIService() error {
	apiService := &v1beta1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "v1beta1.metrics.k8s.io",
		},
		Spec: v1beta1.APIServiceSpec{
			Service: &v1beta1.ServiceReference{
				Name:      "metrics-server",
				Namespace: "kube-system",
			},
			Group:                 "metrics.k8s.io",
			Version:               "v1beta1",
			InsecureSkipTLSVerify: true,
			GroupPriorityMinimum:  100,
			VersionPriority:       30,
		},
	}

	old, err := m.apiregistrationClientset.ApiregistrationV1beta1().APIServices().Get(apiService.GetName(), metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			logrus.Infof("api service(%s) not found, create one.", apiService.GetName())
			_, err = m.apiregistrationClientset.ApiregistrationV1beta1().APIServices().Create(apiService)
			if err != nil {
				return fmt.Errorf("create new api service: %v", err)
			}
			return nil
		}
		return fmt.Errorf("retrieve api service: %v", err)
	}

	logrus.Infof("an old api service(%s) has been found, update it.", apiService.GetName())
	apiService.ResourceVersion = old.ResourceVersion
	if _, err := m.apiregistrationClientset.ApiregistrationV1beta1().APIServices().Update(apiService); err != nil {
		return fmt.Errorf("update api service: %v", err)
	}

	return nil
}

func (m *MetricsServiceManager) newMetricsServerService() error {
	new := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metrics-server",
			Namespace: "kube-system",
			Labels: map[string]string{
				"kubernetes.io/name":            "Metrics-server",
				"kubernetes.io/cluster-service": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString("main-port"),
				},
			},
		},
		Status: corev1.ServiceStatus{},
	}

	old, err := m.clientset.CoreV1().Services(new.Namespace).Get(new.Name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err = m.clientset.CoreV1().Services(new.Namespace).Create(new)
			if err != nil {
				return fmt.Errorf("create new service for : %v", err)
			}
			return nil
		}
		return fmt.Errorf("retrieve service for metrics-server: %v", err)
	}

	new.ResourceVersion = old.ResourceVersion
	new.Spec.ClusterIP = old.Spec.ClusterIP
	_, err = m.clientset.CoreV1().Services(new.Namespace).Update(new)
	if err != nil {
		return fmt.Errorf("update service for metrics-server: %v", err)
	}

	return nil
}

func (m *MetricsServiceManager) newMetricsServiceEndpoints() error {
	endpoints, err := m.listMetricsServiceEndpoints()
	if err != nil {
		return err
	}
	ep := m.metricsServerEndpoint2CoreV1Endpoints(endpoints)
	m.ensureEndpoints(ep)
	go m.watchMetricsServiceEndpoints()
	return nil
}

func (m *MetricsServiceManager) listMetricsServiceEndpoints() ([]metricsServerEndpoint, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp, err := m.clientv3.Get(ctx, "/rainbond/endpoint/METRICS_SERVER_ENDPOINTS", clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("list metrics-server endpoints: %v", err)
	}
	var endpoints []metricsServerEndpoint
	for _, kv := range resp.Kvs {
		eps := m.str2MetricsServerEndpoint(kv)
		endpoints = append(endpoints, eps...)
	}
	return endpoints, nil
}

func (m *MetricsServiceManager) watchMetricsServiceEndpoints() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watchCh := m.clientv3.Watch(ctx, "/rainbond/endpoint/METRICS_SERVER_ENDPOINTS", clientv3.WithPrefix())
	for {
		select {
		case resp := <-watchCh:
			for _, event := range resp.Events {
				eps := m.str2MetricsServerEndpoint(event.Kv)
				ep := m.metricsServerEndpoint2CoreV1Endpoints(eps)
				m.ensureEndpoints(ep)
			}
		}
	}

}

func (m *MetricsServiceManager) str2MetricsServerEndpoint(kv *mvccpb.KeyValue) []metricsServerEndpoint {
	var endpoints []metricsServerEndpoint
	var eps []string
	if err := ffjson.Unmarshal(kv.Value, &eps); err != nil {
		logrus.Warningf("key: %s; value: %s; wrong metrics-server endpoints: %v", kv.Key, kv.Value, err)
		return nil
	}

	for _, ep := range eps {
		ep = strings.Replace(ep, "http://", "", -1)
		ep = strings.Replace(ep, "https://", "", -1)
		epsli := strings.Split(ep, ":")
		if len(epsli) != 2 {
			logrus.Warningf("key: %s; value: %s; wrong metrics-server endpoints.", kv.Key, kv.Value)
			continue
		}
		port, err := strconv.Atoi(epsli[1])
		if err != nil {
			logrus.Warningf("key: %s; value: %s; wrong metrics-server endpoints: %v", kv.Key, kv.Value, err)
			continue
		}
		endpoints = append(endpoints, metricsServerEndpoint{
			Address: epsli[0],
			Port:    port,
		})
	}
	return endpoints
}

func (m *MetricsServiceManager) metricsServerEndpoint2CoreV1Endpoints(endpoints []metricsServerEndpoint) *corev1.Endpoints {
	var subsets []corev1.EndpointSubset
	for _, ep := range endpoints {
		subset := corev1.EndpointSubset{
			Addresses: []corev1.EndpointAddress{{IP: ep.Address}},
			Ports:     []corev1.EndpointPort{{Port: int32(ep.Port)}},
		}
		subsets = append(subsets, subset)
	}
	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metrics-server",
			Namespace: "kube-system",
			Labels: map[string]string{
				"kubernetes.io/name":            "Metrics-server",
				"kubernetes.io/cluster-service": "true",
			},
		},
		Subsets: subsets,
	}
}

func (m *MetricsServiceManager) ensureEndpoints(ep *corev1.Endpoints) {
	old, err := m.clientset.CoreV1().Endpoints(ep.Namespace).Get(ep.Name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err = m.clientset.CoreV1().Endpoints(ep.Namespace).Create(ep)
			if err != nil {
				logrus.Warningf("create endpoints for metrics-server: %v", err)
			}
			return
		}
		logrus.Errorf("retrieve endpoints: %v", err)
		return
	}

	ep.ResourceVersion = old.ResourceVersion
	_, err = m.clientset.CoreV1().Endpoints(ep.Namespace).Update(ep)
	if err != nil {
		logrus.Warningf("update endpoints for metrics-server: %v", err)
	}
}
