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

package appm

import (
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/worker/appm/prober"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/appm/thirdparty"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// NewAPPMController creates a new appm controller.
func NewAPPMController(clientset kubernetes.Interface,
	store store.Storer,
	startCh *channels.RingChannel,
	updateCh *channels.RingChannel,
	probeCh *channels.RingChannel) *Controller {
	c := &Controller{
		store:    store,
		updateCh: updateCh,
		startCh:  startCh,
		probeCh:  probeCh,
		stopCh:   make(chan struct{}),
	}
	// create prober first, then thirdparty
	c.prober = prober.NewProber(c.store, c.probeCh, c.updateCh)
	c.thirdparty = thirdparty.NewThirdPartier(clientset, c.store, c.startCh, c.updateCh, c.stopCh, c.prober)
	return c
}

// Controller describes a new appm controller.
type Controller struct {
	store      store.Storer
	thirdparty thirdparty.ThirdPartier
	prober     prober.Prober

	startCh  *channels.RingChannel
	updateCh *channels.RingChannel
	probeCh  *channels.RingChannel
	stopCh   chan struct{}
}

// Start starts appm controller
func (c *Controller) Start() error {
	c.thirdparty.Start()
	c.prober.Start()
	logrus.Debugf("start thirdparty appm manager success")
	return nil
}

// Stop stops appm controller.
func (c *Controller) Stop() {
	close(c.stopCh)
	c.prober.Stop()
}
