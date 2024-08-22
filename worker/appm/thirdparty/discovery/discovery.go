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

// 该文件定义了Rainbond平台中的服务发现机制接口 `Discoverier` 以及相关的事件类型和处理方法。
// 服务发现机制用于监控和管理第三方服务的端点信息，确保平台能够及时获取和更新这些服务的状态。

// 文件中的主要功能包括：
// 1. `EventType` 类型和常量：定义了服务发现过程中可能发生的事件类型，包括创建、更新、删除、健康和不健康事件。
//    这些事件类型用于标识服务状态的变化，便于对服务的监控和管理。
// 2. `Event` 结构体：封装了一个事件的上下文信息，包括事件的类型和关联的对象。通过该结构体，平台能够详细描述
//    某一特定服务状态变化的细节。
// 3. `Discoverier` 接口：定义了服务发现的基本方法，包括 `Connect` 连接到服务发现中心，`Close` 关闭连接，
//    `Fetch` 获取服务端点信息，以及 `Watch` 监控服务状态变化。该接口为不同类型的服务发现实现提供了统一的操作规范。
// 4. `NewDiscoverier` 工厂方法：根据传入的服务发现配置创建对应类型的 `Discoverier` 实例。目前支持的服务发现类型包括Etcd。
//    如果传入了不支持的服务发现类型，该方法将返回一个错误。

// 总的来说，该文件通过定义服务发现接口和事件类型，为Rainbond平台提供了统一的服务发现机制，
// 能够监控和管理第三方服务的状态。这对于平台能够动态调整和优化服务的使用至关重要，确保了服务的高可用性和稳定性。

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
	DeleteEvent EventType = "DELETE"
	// UnhealthyEvent -
	UnhealthyEvent EventType = "UNHEALTHY"
	// HealthEvent -
	HealthEvent EventType = "HEALTH"
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
