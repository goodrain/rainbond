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
