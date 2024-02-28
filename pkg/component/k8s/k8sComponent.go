// RAINBOND, Application Management Platform
// Copyright (C) 2021-2024 Goodrain Co., Ltd.

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

package k8s

import (
	"context"
	apisixversioned "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	rainbondscheme "github.com/goodrain/rainbond/pkg/generated/clientset/versioned/scheme"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	kruiseclientset "github.com/openkruise/kruise-api/client/clientset/versioned"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
	"kubevirt.io/client-go/kubecli"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1beta1"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1beta1"
)

// Component -
type Component struct {
	RestConfig    *rest.Config
	Clientset     *kubernetes.Clientset
	GatewayClient *v1beta1.GatewayV1beta1Client
	DynamicClient *dynamic.DynamicClient

	RainbondClient *versioned.Clientset
	K8sClient      k8sclient.Client
	KubevirtCli    kubecli.KubevirtClient

	Mapper meta.RESTMapper

	ApiSixClient *apisixversioned.Clientset
	KruiseClient *kruiseclientset.Clientset
	MetricClient *metrics.Clientset
}

var defaultK8sComponent *Component

// Client -
func Client() *Component {
	defaultK8sComponent = &Component{}
	return defaultK8sComponent
}

// Start -
func (k *Component) Start(ctx context.Context, cfg *configs.Config) error {
	logrus.Infof("init k8s client...")
	config, err := k8sutil.NewRestConfig(cfg.APIConfig.KubeConfigPath)
	k.RestConfig = config
	if err != nil {
		logrus.Errorf("create k8s config failure: %v", err)
		return err
	}
	k.Clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Errorf("create k8s client failure: %v", err)
		return err
	}
	k.GatewayClient, err = gateway.NewForConfig(config)
	if err != nil {
		logrus.Errorf("create gateway client failure: %v", err)
		return err
	}
	k.DynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		logrus.Errorf("create dynamic client failure: %v", err)
		return err
	}
	k.KruiseClient = kruiseclientset.NewForConfigOrDie(config)

	k.ApiSixClient, err = apisixversioned.NewForConfig(config)
	if err != nil {
		logrus.Errorf("create apisix clientset error, %v", err)
		return err
	}

	k.MetricClient, err = metrics.NewForConfig(config)
	if err != nil {
		return err
	}

	k.RainbondClient = versioned.NewForConfigOrDie(config)

	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)
	rainbondscheme.AddToScheme(scheme)
	k.K8sClient, err = k8sclient.New(config, k8sclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		logrus.Errorf("create k8s client failure: %v", err)
		return err
	}

	k.KubevirtCli, err = kubecli.GetKubevirtClientFromRESTConfig(config)
	if err != nil {
		logrus.Errorf("create kubevirt cli failure: %v", err)
		return err
	}

	gr, err := restmapper.GetAPIGroupResources(k.Clientset)
	if err != nil {
		return err
	}
	k.Mapper = restmapper.NewDiscoveryRESTMapper(gr)
	logrus.Infof("init k8s client success")
	return nil
}

// CloseHandle -
func (k *Component) CloseHandle() {
}

// Default -
func Default() *Component {
	return defaultK8sComponent
}
