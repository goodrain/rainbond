// RAINBOND, Application Management Platform
// Copyright (C) 2021-2021 Goodrain Co., Ltd.

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
// 本文件定义了 Rainbond 平台中控制器（Controller）接口的基础结构，用于在应用管理平台中统一管理和监控控制器的行为。控制器是负责协调资源和执行操作的核心组件。

// 文件的主要功能如下：

// 1. `Controller` 接口：
//    - `Controller` 接口继承了 `reconcile.Reconciler` 接口，这意味着实现该接口的控制器必须实现 `Reconcile` 方法，该方法负责处理资源的调和逻辑。
//    - `Collect` 方法用于收集 Prometheus 指标，`ch` 通道用于传输收集到的指标数据。通过该方法，控制器可以将其内部状态和性能数据公开给 Prometheus，用于监控和分析。

// 2. `reconcile.Reconciler` 接口：
//    - `Reconciler` 是 Kubernetes 控制器运行时中的一个核心接口，定义了资源调和的行为。实现该接口的对象可以处理集群中资源的变更，并根据需要执行相应的操作。

// 3. `Collect` 方法：
//    - `Collect` 方法为控制器提供了与 Prometheus 进行集成的能力。控制器可以通过该方法暴露自定义的指标数据，供 Prometheus 进行抓取和监控。
//    - 这是 Prometheus 客户端库提供的一种标准机制，允许应用程序公开其内部状态，以供监控系统使用。

// 综上所述，本文件定义了一个用于在 Rainbond 平台中实现控制器的接口结构。通过这个接口，开发人员可以创建符合平台需求的控制器，执行资源调和操作，并将控制器的运行状态和性能数据集成到 Prometheus 监控系统中。这种设计有助于平台保持资源的稳定性和高可用性。

package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Controller interface {
	reconcile.Reconciler
	Collect(ch chan<- prometheus.Metric)
}
