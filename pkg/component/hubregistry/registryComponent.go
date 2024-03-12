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

package hubregistry

import (
	"context"
	rainbondv1alpha1 "github.com/goodrain/rainbond-operator/api/v1alpha1"
	"github.com/goodrain/rainbond/builder/sources/registry"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/gogo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

var defaultRegistryComponent *RegistryComponent

// RegistryComponent -
type RegistryComponent struct {
	RegistryCli *registry.Registry
}

// HubRegistry -
func HubRegistry() *RegistryComponent {
	defaultRegistryComponent = &RegistryComponent{}
	return defaultRegistryComponent
}

// Start -
func (r *RegistryComponent) Start(ctx context.Context, cfg *configs.Config) error {
	logrus.Infof("init hub registry...")
	var cluster rainbondv1alpha1.RainbondCluster

	err := clients.K8SClientInitClient(k8s.Default().Clientset, k8s.Default().RestConfig)
	if err != nil {
		logrus.Errorf("k8s client init rainbondClient failure: %v", err)
		return err
	}
	if err := clients.RainbondKubeClient.Get(context.Background(), types.NamespacedName{Namespace: "rbd-system", Name: "rainbondcluster"}, &cluster); err != nil {
		return errors.Wrap(err, "get configuration from rainbond cluster")
	}

	registryConfig := cluster.Spec.ImageHub
	if registryConfig.Domain == "goodrain.me" {
		registryConfig.Domain = cfg.APIConfig.RbdHub
	}
	gogo.Go(func(ctx context.Context) error {
		for {
			logrus.Info("初始化镜像仓库 ", registryConfig.Domain, registryConfig.Username, registryConfig.Password)
			r.RegistryCli, err = registry.NewInsecure(registryConfig.Domain, registryConfig.Username, registryConfig.Password)
			if err == nil {
				logrus.Infof("create hub client success")
				return nil
			}
			logrus.Errorf("create hub client failed, try time is %d,%s", 10, err.Error())
			time.Sleep(10 * time.Second)
		}
	})
	return nil
}

// CloseHandle -
func (r *RegistryComponent) CloseHandle() {

}

// Default -
func Default() *RegistryComponent {
	return defaultRegistryComponent
}
