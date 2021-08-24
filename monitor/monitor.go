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

package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/goodrain/rainbond/cmd/monitor/option"
	"github.com/goodrain/rainbond/monitor/prometheus"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	externalversions "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	"github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//Monitor monitor
type Monitor struct {
	config             *option.Config
	ctx                context.Context
	cancel             context.CancelFunc
	manager            *prometheus.Manager
	prometheusOperator *prometheus.PrometheusOperator
	stopCh             chan struct{}
}

//Start start
func (d *Monitor) Start() {
	logrus.Info("start init prometheus operator")
	d.prometheusOperator.Run(d.stopCh)
	logrus.Info("init prometheus operator success")
}

// Stop stop monitor
func (d *Monitor) Stop() {
	d.prometheusOperator.Stop()
	d.cancel()
	close(d.stopCh)
	logrus.Info("prometheus operator stoped")
}

// NewMonitor new monitor
func NewMonitor(opt *option.Config, p *prometheus.Manager) (*Monitor, error) {
	restConfig, err := k8sutil.NewRestConfig(opt.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("new kube api rest config failure %s", err.Error())
	}
	ctx, cancel := context.WithCancel(context.Background())
	d := &Monitor{
		config:  opt,
		ctx:     ctx,
		cancel:  cancel,
		manager: p,
		stopCh:  make(chan struct{}),
	}
	c, err := versioned.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	smFactory := externalversions.NewSharedInformerFactoryWithOptions(c, 5*time.Minute,
		externalversions.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = "creator=Rainbond"
		}))
	d.prometheusOperator, err = prometheus.NewPrometheusOperator(ctx, smFactory, p)
	if err != nil {
		return nil, err
	}
	return d, nil
}
