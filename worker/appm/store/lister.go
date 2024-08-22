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

// 该文件定义了Rainbond平台中用于管理和缓存Kubernetes资源对象的Lister结构体。
// 通过集成多个Kubernetes资源的Lister，该文件为Rainbond平台提供了对Kubernetes集群中各种资源的
// 实时查询和访问能力。

// 文件中的主要功能包括：
// 1. `Lister` 结构体：定义了多个Kubernetes资源的Lister，包括Ingress、Service、Secret、StatefulSet、
//    Deployment、Pod、ReplicaSets、ConfigMap、Endpoints、Nodes、StorageClass、PersistentVolumeClaim、
//    HorizontalPodAutoscaler、CustomResourceDefinition (CRD)、HelmApp、ComponentDefinition、ThirdComponent、
//    Job、CronJob等。这些Lister用于查询和缓存Kubernetes集群中相应资源的状态和信息。
// 2. 资源管理：通过这些Lister，Rainbond平台能够高效地访问和操作Kubernetes中的资源，
//    提供了更快速和简便的资源查询能力，避免了每次查询都直接访问API Server，从而减少了对Kubernetes集群的负载。

// 总的来说，该文件通过定义和管理Kubernetes资源的Lister，使Rainbond平台能够高效地管理和访问
// 集群中的资源信息，从而实现对应用服务的实时监控和高效管理。这对于确保平台的稳定性和响应能力至关重要。

package store

import (
	"github.com/goodrain/rainbond/pkg/generated/listers/rainbond/v1alpha1"
	crdlisters "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	appsv1 "k8s.io/client-go/listers/apps/v1"
	autoscalingv2 "k8s.io/client-go/listers/autoscaling/v2"
	autoscalingv2beta2 "k8s.io/client-go/listers/autoscaling/v2beta2"
	v1 "k8s.io/client-go/listers/batch/v1"
	batchv1beta1 "k8s.io/client-go/listers/batch/v1beta1"
	corev1 "k8s.io/client-go/listers/core/v1"
	networkingv1 "k8s.io/client-go/listers/networking/v1"
	betav1 "k8s.io/client-go/listers/networking/v1beta1"
	storagev1 "k8s.io/client-go/listers/storage/v1"
)

// Lister kube-api client cache
type Lister struct {
	Ingress                      networkingv1.IngressLister
	BetaIngress                  betav1.IngressLister
	Service                      corev1.ServiceLister
	Secret                       corev1.SecretLister
	StatefulSet                  appsv1.StatefulSetLister
	Deployment                   appsv1.DeploymentLister
	Pod                          corev1.PodLister
	ReplicaSets                  appsv1.ReplicaSetLister
	ConfigMap                    corev1.ConfigMapLister
	Endpoints                    corev1.EndpointsLister
	Nodes                        corev1.NodeLister
	StorageClass                 storagev1.StorageClassLister
	Claims                       corev1.PersistentVolumeClaimLister
	HorizontalPodAutoscaler      autoscalingv2.HorizontalPodAutoscalerLister
	HorizontalPodAutoscalerbeta2 autoscalingv2beta2.HorizontalPodAutoscalerLister
	CRD                          crdlisters.CustomResourceDefinitionLister
	HelmApp                      v1alpha1.HelmAppLister
	ComponentDefinition          v1alpha1.ComponentDefinitionLister
	ThirdComponent               v1alpha1.ThirdComponentLister
	Job                          v1.JobLister
	CronJob                      v1.CronJobLister
	BetaCronJob                  batchv1beta1.CronJobLister
}
