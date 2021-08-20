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

package service

import (
	"context"
	"fmt"
	"strings"

	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/cmd/node-proxy/option"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//DiscoverAction DiscoverAction
type DiscoverAction struct {
	conf    *option.Conf
	kubecli *kubernetes.Clientset
}

//CreateDiscoverActionManager CreateDiscoverActionManager
func CreateDiscoverActionManager(conf *option.Conf, kubecli *kubernetes.Clientset) *DiscoverAction {
	return &DiscoverAction{
		conf:    conf,
		kubecli: kubecli,
	}
}

//GetPluginConfigs get plugin configs
//if not exist return error
func (d *DiscoverAction) GetPluginConfigs(ctx context.Context, namespace, sourceAlias, pluginID string) (*api_model.ResourceSpec, error) {
	labelname := fmt.Sprintf("plugin_id=%s,service_alias=%s", pluginID, sourceAlias)
	configs, err := d.kubecli.CoreV1().ConfigMaps(namespace).List(ctx, v1.ListOptions{LabelSelector: labelname})
	if err != nil {
		return nil, fmt.Errorf("get plugin config failure %s", err.Error())
	}
	if len(configs.Items) == 0 {
		return nil, nil
	}
	var rs api_model.ResourceSpec
	if err := ffjson.Unmarshal([]byte(configs.Items[0].Data["plugin-config"]), &rs); err != nil {
		logrus.Errorf("unmashal etcd v error, %v", err)
		return nil, err
	}
	return &rs, nil
}

//GetPluginConfigAndType get plugin configs and plugin type (default mesh or custom mesh)
//if not exist return error
func (d *DiscoverAction) GetPluginConfigAndType(ctx context.Context, namespace, sourceAlias, pluginID string) (*api_model.ResourceSpec, bool, error) {
	labelname := fmt.Sprintf("plugin_id=%s,service_alias=%s", pluginID, sourceAlias)
	configs, err := d.kubecli.CoreV1().ConfigMaps(namespace).List(ctx, v1.ListOptions{LabelSelector: labelname})
	if err != nil {
		return nil, false, fmt.Errorf("get plugin config failure %s", err.Error())
	}
	if len(configs.Items) == 0 {
		return nil, false, nil
	}
	var rs api_model.ResourceSpec
	if err := ffjson.Unmarshal([]byte(configs.Items[0].Data["plugin-config"]), &rs); err != nil {
		logrus.Errorf("unmashal etcd v error, %v", err)
		return nil, strings.Contains(configs.Items[0].Name, "def-mesh"), err
	}
	return &rs, strings.Contains(configs.Items[0].Name, "def-mesh"), nil
}
