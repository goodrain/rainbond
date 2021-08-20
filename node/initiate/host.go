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

// thanks @lextoumbourou.

package initiate

import (
	"context"
	"errors"

	"github.com/goodrain/rainbond/cmd/node-proxy/option"
	discover "github.com/goodrain/rainbond/discover.v2"
	"github.com/goodrain/rainbond/discover/config"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
)

var (
	// ErrRegistryAddressNotFound record not found error, happens when haven't find any matched registry address.
	ErrRegistryAddressNotFound = errors.New("registry address not found")
)

// HostManager is responsible for writing the resolution of the private image repository domain name to /etc/hosts.
type HostManager interface {
	Start()
}

// NewHostManager creates a new HostManager.
func NewHostManager(cfg *option.Conf, discover discover.Discover) (HostManager, error) {
	hosts, err := util.NewHosts(cfg.HostsFile)
	if err != nil {
		return nil, err
	}
	callback := &hostCallback{
		cfg:   cfg,
		hosts: hosts,
	}
	return &hostManager{
		cfg:          cfg,
		discover:     discover,
		hostCallback: callback,
	}, nil
}

type hostManager struct {
	ctx          context.Context
	cfg          *option.Conf
	discover     discover.Discover
	hostCallback *hostCallback
}

func (h *hostManager) Start() {
	if h.cfg.ImageRepositoryHost == "" {
		// no need to write hosts file
		return
	}
	if h.cfg.GatewayVIP != "" {
		if err := h.hostCallback.hosts.Cleanup(); err != nil {
			logrus.Warningf("cleanup hosts file: %v", err)
			return
		}
		logrus.Infof("set hosts %s to %s", h.cfg.ImageRepositoryHost, h.cfg.GatewayVIP)
		lines := []string{
			util.StartOfSection,
			h.cfg.GatewayVIP + " " + h.cfg.ImageRepositoryHost,
			h.cfg.GatewayVIP + " " + "region.goodrain.me",
			util.EndOfSection,
		}
		h.hostCallback.hosts.AddLines(lines...)
		if err := h.hostCallback.hosts.Flush(); err != nil {
			logrus.Warningf("flush hosts file: %v", err)
		}
		return
	}
	h.discover.AddProject("rbd-gateway", h.hostCallback)
}

type hostCallback struct {
	cfg   *option.Conf
	hosts util.Hosts
}

// TODO HA error
func (h *hostCallback) UpdateEndpoints(endpoints ...*config.Endpoint) {
	logrus.Info("hostCallback; update endpoints")
	if err := h.hosts.Cleanup(); err != nil {
		logrus.Warningf("cleanup hosts file: %v", err)
		return
	}

	if len(endpoints) > 0 {
		logrus.Infof("found endpints: %d; endpoint selected: %#v", len(endpoints), *endpoints[0])
		lines := []string{
			util.StartOfSection,
			endpoints[0].URL + " " + h.cfg.ImageRepositoryHost,
			endpoints[0].URL + " " + "region.goodrain.me",
			util.EndOfSection,
		}
		h.hosts.AddLines(lines...)
	}

	if err := h.hosts.Flush(); err != nil {
		logrus.Warningf("flush hosts file: %v", err)
	}
}

func (h *hostCallback) Error(err error) {
	logrus.Warningf("unexpected error from host callback: %v", err)
}
