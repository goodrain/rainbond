// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package clients

import (
	"fmt"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/rest"
	"os"
	"path"

	rainbondv1alpha1 "github.com/goodrain/rainbond-operator/api/v1alpha1"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(rainbondv1alpha1.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
}

//K8SClient K8SClient
var K8SClient kubernetes.Interface

//RainbondKubeClient rainbond custom resource client
var RainbondKubeClient client.Client

//InitClient init k8s client
func InitClient(kubeconfig string) error {
	if kubeconfig == "" {
		homePath, _ := sources.Home()
		kubeconfig = path.Join(homePath, ".kube/config")
	}
	var config *rest.Config
	_, err := os.Stat(kubeconfig)
	if err != nil {
		fmt.Printf("Not find kube-config file(%s)\n", kubeconfig)
		if config, err = rest.InClusterConfig(); err != nil{
			logrus.Error("get cluster config error:", err)
			return err
		}
	} else {
		// use the current context in kubeconfig
		config, err = k8sutil.NewRestConfig(kubeconfig)
		if err != nil {
			return err
		}
	}
	config.QPS = 50
	config.Burst = 100

	K8SClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Error("Create kubernetes client error.", err.Error())
		return err
	}
	mapper, err := apiutil.NewDynamicRESTMapper(config, apiutil.WithLazyDiscovery)
	if err != nil {
		return fmt.Errorf("NewDynamicRESTMapper failure %+v", err)
	}
	runtimeClient, err := client.New(config, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		return fmt.Errorf("New kube client failure %+v", err)
	}
	RainbondKubeClient = runtimeClient
	return nil
}
