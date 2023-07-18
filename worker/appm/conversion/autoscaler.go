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

package conversion

import (
	"fmt"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	utilversion "k8s.io/apimachinery/pkg/util/version"

	"github.com/sirupsen/logrus"

	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

var str2ResourceName = map[string]corev1.ResourceName{
	"cpu":    corev1.ResourceCPU,
	"memory": corev1.ResourceMemory,
}

// TenantServiceAutoscaler -
func TenantServiceAutoscaler(as *v1.AppService, dbmanager db.Manager) error {
	if k8sutil.GetKubeVersion().AtLeast(utilversion.MustParseSemantic("v1.23.0")) {
		hpas, err := newHPAs(as, dbmanager)
		if err != nil {
			return fmt.Errorf("create HPAs: %v", err)
		}
		logrus.Debugf("the numbers of HPAs: %d", len(hpas))

		as.SetHPAs(hpas)

	} else {
		hpas, err := newHPABeta2s(as, dbmanager)
		if err != nil {
			return fmt.Errorf("create Beta2 HPAs: %v", err)
		}
		logrus.Debugf("the numbers of Beta2 HPAs: %d", len(hpas))

		as.SetHPAbeta2s(hpas)

	}
	return nil
}

func newHPABeta2s(as *v1.AppService, dbmanager db.Manager) ([]*autoscalingv2beta2.HorizontalPodAutoscaler, error) {
	xpaRules, err := dbmanager.TenantServceAutoscalerRulesDao().ListEnableOnesByServiceID(as.ServiceID)
	if err != nil {
		return nil, err
	}

	var hpas []*autoscalingv2beta2.HorizontalPodAutoscaler
	for _, rule := range xpaRules {
		metrics, err := dbmanager.TenantServceAutoscalerRuleMetricsDao().ListByRuleID(rule.RuleID)
		if err != nil {
			return nil, err
		}

		var kind, name string
		if as.GetStatefulSet() != nil {
			kind, name = "StatefulSet", as.GetStatefulSet().GetName()
		} else {
			kind, name = "Deployment", as.GetDeployment().GetName()
		}

		labels := as.GetCommonLabels(map[string]string{
			"rule_id": rule.RuleID,
			"version": as.DeployVersion,
		})

		hpa := newHPABeta2(as.GetNamespace(), kind, name, labels, rule, metrics)

		hpas = append(hpas, hpa)
	}

	return hpas, nil
}

func createResourceMetricsBeta2(metric *model.TenantServiceAutoscalerRuleMetrics) autoscalingv2beta2.MetricSpec {
	ms := autoscalingv2beta2.MetricSpec{
		Type: autoscalingv2beta2.ResourceMetricSourceType,
		Resource: &autoscalingv2beta2.ResourceMetricSource{
			Name: str2ResourceName[metric.MetricsName],
		},
	}

	if metric.MetricTargetType == "utilization" {
		value := int32(metric.MetricTargetValue)
		ms.Resource.Target = autoscalingv2beta2.MetricTarget{
			Type:               autoscalingv2beta2.UtilizationMetricType,
			AverageUtilization: &value,
		}
	}
	if metric.MetricTargetType == "average_value" {
		ms.Resource.Target.Type = autoscalingv2beta2.AverageValueMetricType
		if metric.MetricsName == "cpu" {
			ms.Resource.Target.AverageValue = resource.NewMilliQuantity(int64(metric.MetricTargetValue), resource.DecimalSI)
		}
		if metric.MetricsName == "memory" {
			ms.Resource.Target.AverageValue = resource.NewQuantity(int64(metric.MetricTargetValue*1024*1024), resource.BinarySI)
		}
	}

	return ms
}

func newHPABeta2(namespace, kind, name string, labels map[string]string, rule *model.TenantServiceAutoscalerRules, metrics []*model.TenantServiceAutoscalerRuleMetrics) *autoscalingv2beta2.HorizontalPodAutoscaler {
	hpa := &autoscalingv2beta2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rule.RuleID,
			Namespace: namespace,
			Labels:    labels,
		},
	}

	spec := autoscalingv2beta2.HorizontalPodAutoscalerSpec{
		MinReplicas: util.Int32(int32(rule.MinReplicas)),
		MaxReplicas: int32(rule.MaxReplicas),
		ScaleTargetRef: autoscalingv2beta2.CrossVersionObjectReference{
			Kind:       kind,
			Name:       name,
			APIVersion: "apps/v1",
		},
	}

	for _, metric := range metrics {
		if metric.MetricsType != "resource_metrics" {
			logrus.Warningf("rule id:  %s; unsupported metric type: %s", rule.RuleID, metric.MetricsType)
			continue
		}
		if metric.MetricTargetValue <= 0 {
			// TODO: If the target value of cpu and memory is 0, it will not take effect.
			// TODO: The target value of the custom indicator can be 0.
			continue
		}

		ms := createResourceMetricsBeta2(metric)
		spec.Metrics = append(spec.Metrics, ms)
	}
	if len(spec.Metrics) == 0 {
		return nil
	}
	hpa.Spec = spec

	return hpa
}

func newHPAs(as *v1.AppService, dbmanager db.Manager) ([]*autoscalingv2.HorizontalPodAutoscaler, error) {
	xpaRules, err := dbmanager.TenantServceAutoscalerRulesDao().ListEnableOnesByServiceID(as.ServiceID)
	if err != nil {
		return nil, err
	}

	var hpas []*autoscalingv2.HorizontalPodAutoscaler
	for _, rule := range xpaRules {
		metrics, err := dbmanager.TenantServceAutoscalerRuleMetricsDao().ListByRuleID(rule.RuleID)
		if err != nil {
			return nil, err
		}

		var kind, name string
		if as.GetStatefulSet() != nil {
			kind, name = "StatefulSet", as.GetStatefulSet().GetName()
		} else {
			kind, name = "Deployment", as.GetDeployment().GetName()
		}

		labels := as.GetCommonLabels(map[string]string{
			"rule_id": rule.RuleID,
			"version": as.DeployVersion,
		})

		hpa := newHPA(as.GetNamespace(), kind, name, labels, rule, metrics)

		hpas = append(hpas, hpa)
	}

	return hpas, nil
}

func createResourceMetrics(metric *model.TenantServiceAutoscalerRuleMetrics) autoscalingv2.MetricSpec {
	ms := autoscalingv2.MetricSpec{
		Type: autoscalingv2.ResourceMetricSourceType,
		Resource: &autoscalingv2.ResourceMetricSource{
			Name: str2ResourceName[metric.MetricsName],
		},
	}

	if metric.MetricTargetType == "utilization" {
		value := int32(metric.MetricTargetValue)
		ms.Resource.Target = autoscalingv2.MetricTarget{
			Type:               autoscalingv2.UtilizationMetricType,
			AverageUtilization: &value,
		}
	}
	if metric.MetricTargetType == "average_value" {
		ms.Resource.Target.Type = autoscalingv2.AverageValueMetricType
		if metric.MetricsName == "cpu" {
			ms.Resource.Target.AverageValue = resource.NewMilliQuantity(int64(metric.MetricTargetValue), resource.DecimalSI)
		}
		if metric.MetricsName == "memory" {
			ms.Resource.Target.AverageValue = resource.NewQuantity(int64(metric.MetricTargetValue*1024*1024), resource.BinarySI)
		}
	}

	return ms
}

func newHPA(namespace, kind, name string, labels map[string]string, rule *model.TenantServiceAutoscalerRules, metrics []*model.TenantServiceAutoscalerRuleMetrics) *autoscalingv2.HorizontalPodAutoscaler {
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rule.RuleID,
			Namespace: namespace,
			Labels:    labels,
		},
	}

	spec := autoscalingv2.HorizontalPodAutoscalerSpec{
		MinReplicas: util.Int32(int32(rule.MinReplicas)),
		MaxReplicas: int32(rule.MaxReplicas),
		ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
			Kind:       kind,
			Name:       name,
			APIVersion: "apps/v1",
		},
	}

	for _, metric := range metrics {
		if metric.MetricsType != "resource_metrics" {
			logrus.Warningf("rule id:  %s; unsupported metric type: %s", rule.RuleID, metric.MetricsType)
			continue
		}
		if metric.MetricTargetValue <= 0 {
			// TODO: If the target value of cpu and memory is 0, it will not take effect.
			// TODO: The target value of the custom indicator can be 0.
			continue
		}

		ms := createResourceMetrics(metric)
		spec.Metrics = append(spec.Metrics, ms)
	}
	if len(spec.Metrics) == 0 {
		return nil
	}
	hpa.Spec = spec

	return hpa
}
