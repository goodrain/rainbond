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

package discovery

import (
	"fmt"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	"strings"
)

// EventType type of event
type EventType string

const (
	// CreateEvent event associated with new objects in a service discovery center
	CreateEvent EventType = "CREATE"
	// UpdateEvent event associated with an object update in a service discovery center
	UpdateEvent EventType = "UPDATE"
	// DeleteEvent event associated when an object is removed from a service discovery center
	DeleteEvent    EventType = "DELETE"
	UnhealthyEvent EventType = "UNHEALTHY"
	HealthEvent    EventType = "HEALTH"
)

// Event holds the context of an event.
type Event struct {
	Type EventType
	Obj  interface{}
}

// Discoverier is the interface that wraps the required methods to gather
// information about third-party service endpoints.
type Discoverier interface {
	Connect() error
	Close() error
	Fetch() ([]*v1.RbdEndpoint, error)
	Watch()
}

// NewDiscoverier creates a new Discoverier.
func NewDiscoverier(cfg *model.ThirdPartySvcDiscoveryCfg,
	updateCh *channels.RingChannel,
	stopCh chan struct{}) (Discoverier, error) {
	switch strings.ToLower(cfg.Type) {
	case strings.ToLower(string(model.DiscorveryTypeEtcd)):
		return NewEtcd(cfg, updateCh, stopCh), nil
	default:
		return nil, fmt.Errorf("Unsupported discovery type: %s", cfg.Type)
	}
}
