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
	"strings"
	"time"

	rainbondv1alpha1 "github.com/goodrain/rainbond-operator/api/v1alpha1"
	"github.com/goodrain/rainbond-operator/util/constants"
	"github.com/goodrain/rainbond/builder/sources/registry"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/gogo"
	utils "github.com/goodrain/rainbond/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
)

var defaultRegistryComponent *RegistryComponent

// RegistryComponent -
type RegistryComponent struct {
	RegistryCli  *registry.Registry
	ServerConfig *configs.ServerConfig
}

// New -
func New() *RegistryComponent {
	defaultRegistryComponent = &RegistryComponent{
		RegistryCli:  new(registry.Registry),
		ServerConfig: configs.Default().ServerConfig,
	}
	return defaultRegistryComponent
}

// Start -
func (r *RegistryComponent) Start(ctx context.Context) error {
	var cluster rainbondv1alpha1.RainbondCluster

	err := clients.K8SClientInitClient(k8s.Default().Clientset, k8s.Default().RestConfig)
	if err != nil {
		logrus.Errorf("k8s client init rainbondClient failure: %v", err)
		return err
	}
	if err := clients.RainbondKubeClient.Get(context.Background(), types.NamespacedName{Namespace: utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace), Name: "rainbondcluster"}, &cluster); err != nil {
		return errors.Wrap(err, "get configuration from rainbond cluster")
	}

	registryConfig := cluster.Spec.ImageHub
	// 提取域名部分(去除端口),支持 "goodrain.me" 和 "goodrain.me:9443" 格式
	domain := registryConfig.Domain
	if strings.Contains(registryConfig.Domain, ":") {
		domain = strings.Split(registryConfig.Domain, ":")[0]
	}
	// 如果是默认域名 goodrain.me,替换为实际的 RbdHub 配置
	if domain == "goodrain.me" {
		registryConfig.Domain = r.ServerConfig.RbdHub
	}
	gogo.Go(func(ctx context.Context) error {
		for {
			registryCli, err := registry.NewInsecure(registryConfig.Domain, registryConfig.Username, registryConfig.Password)
			if err == nil {
				*r.RegistryCli = *registryCli
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
