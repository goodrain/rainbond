/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
/*
本文件实现了与Kubernetes持久卷（Persistent Volume, PV）控制器相关的Prometheus监控指标。这些指标用于监控和度量PV的创建和删除操作的性能和成功率。

1. **ControllerSubsystem**:
   - `ControllerSubsystem` 是一个字符串常量，用作Prometheus子系统名称，用于标识这些监控指标是与PV控制器相关的。

2. **Prometheus指标**:
   - 本文件定义了一系列的Prometheus指标，用于监控与PV创建和删除操作相关的事件。以下是这些指标的详细描述：

   - `PersistentVolumeClaimProvisionTotal`:
     - 这是一个计数器（Counter），用于记录成功创建的持久卷（PV）的总数。该计数器以存储类（StorageClass）为维度进行细分。

   - `PersistentVolumeClaimProvisionFailedTotal`:
     - 这是一个计数器，用于记录创建PV失败的总次数。它以存储类为维度进行细分，帮助识别哪些存储类可能存在问题。

   - `PersistentVolumeClaimProvisionDurationSeconds`:
     - 这是一个直方图（Histogram），用于记录创建PV操作的时延（以秒为单位）。该指标以存储类为维度进行细分，帮助分析不同存储类在创建PV时的性能表现。

   - `PersistentVolumeDeleteTotal`:
     - 这是一个计数器，用于记录成功删除的持久卷的总数。该指标以存储类为维度进行细分。

   - `PersistentVolumeDeleteFailedTotal`:
     - 这是一个计数器，用于记录删除PV失败的总次数。它以存储类为维度进行细分，帮助识别删除操作中可能存在的问题。

   - `PersistentVolumeDeleteDurationSeconds`:
     - 这是一个直方图，用于记录删除PV操作的时延（以秒为单位）。该指标以存储类为维度进行细分，帮助分析不同存储类在删除PV时的性能表现。

3. **Prometheus指标的使用**:
   - 这些指标可以通过Prometheus抓取，以提供关于PV操作的实时监控和告警能力。
   - 例如，通过`PersistentVolumeClaimProvisionFailedTotal`可以监控某一存储类下PV创建的失败次数，并在失败次数过多时触发告警。
   - 通过`PersistentVolumeClaimProvisionDurationSeconds`可以监控PV创建的时延，并优化存储类的配置以降低时延。

总结：
本文件定义了一组用于Kubernetes PV控制器的Prometheus监控指标，帮助运维人员监控PV的创建和删除操作的性能和成功率。这些指标可以用于实时监控、性能分析以及故障排除。
*/

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// ControllerSubsystem is prometheus subsystem name.
	ControllerSubsystem = "controller"
)

var (
	// PersistentVolumeClaimProvisionTotal is used to collect accumulated count of persistent volumes provisioned.
	PersistentVolumeClaimProvisionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: ControllerSubsystem,
			Name:      "persistentvolumeclaim_provision_total",
			Help:      "Total number of persistent volumes provisioned. Broken down by storage class name.",
		},
		[]string{"class"},
	)
	// PersistentVolumeClaimProvisionFailedTotal is used to collect accumulated count of persistent volume provision failed attempts.
	PersistentVolumeClaimProvisionFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: ControllerSubsystem,
			Name:      "persistentvolumeclaim_provision_failed_total",
			Help:      "Total number of persistent volume provision failed attempts. Broken down by storage class name.",
		},
		[]string{"class"},
	)
	// PersistentVolumeClaimProvisionDurationSeconds is used to collect latency in seconds to provision persistent volumes.
	PersistentVolumeClaimProvisionDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: ControllerSubsystem,
			Name:      "persistentvolumeclaim_provision_duration_seconds",
			Help:      "Latency in seconds to provision persistent volumes. Broken down by storage class name.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"class"},
	)
	// PersistentVolumeDeleteTotal is used to collect accumulated count of persistent volumes deleted.
	PersistentVolumeDeleteTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: ControllerSubsystem,
			Name:      "persistentvolume_delete_total",
			Help:      "Total number of persistent volumes deleteed. Broken down by storage class name.",
		},
		[]string{"class"},
	)
	// PersistentVolumeDeleteFailedTotal is used to collect accumulated count of persistent volume delete failed attempts.
	PersistentVolumeDeleteFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: ControllerSubsystem,
			Name:      "persistentvolume_delete_failed_total",
			Help:      "Total number of persistent volume delete failed attempts. Broken down by storage class name.",
		},
		[]string{"class"},
	)
	// PersistentVolumeDeleteDurationSeconds is used to collect latency in seconds to delete persistent volumes.
	PersistentVolumeDeleteDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: ControllerSubsystem,
			Name:      "persistentvolume_delete_duration_seconds",
			Help:      "Latency in seconds to delete persistent volumes. Broken down by storage class name.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"class"},
	)
)
