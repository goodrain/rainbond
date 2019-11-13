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

	"github.com/Sirupsen/logrus"

	v2beta1 "k8s.io/api/autoscaling/v2beta1"
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
	hpas, err := newHPAs(as, dbmanager)
	if err != nil {
		return fmt.Errorf("create HPAs: %v", err)
	}

	as.SetHPAs(hpas)

	return nil
}

func newHPAs(as *v1.AppService, dbmanager db.Manager) ([]*v2beta1.HorizontalPodAutoscaler, error) {
	xpaRules, err := dbmanager.TenantServceAutoscalerRulesDao().ListByServiceID(as.ServiceID)
	if err != nil {
		return nil, err
	}

	var hpas []*v2beta1.HorizontalPodAutoscaler
	for _, rule := range xpaRules {
		metrics, err := dbmanager.TenantServceAutoscalerRuleMetricsDao().ListByRuleID(rule.RuleID)
		if err != nil {
			return nil, err
		}

		var kind, name string
		if as.GetStatefulSet() != nil {
			kind, name = "Statefulset", as.GetStatefulSet().GetName()
		} else {
			kind, name = "Deployment", as.GetDeployment().GetName()
		}

		labels := as.GetCommonLabels(map[string]string{
			"rule_id": rule.RuleID,
		})

		hpa := newHPA(as.TenantID, kind, name, labels, rule, metrics)

		hpas = append(hpas, hpa)
	}

	return hpas, nil
}

func createResourceMetrics(metric *model.TenantServiceAutoscalerRuleMetrics) v2beta1.MetricSpec {
	ms := v2beta1.MetricSpec{
		Type: v2beta1.ResourceMetricSourceType,
		Resource: &v2beta1.ResourceMetricSource{
			Name: str2ResourceName[metric.MetricsName],
		},
	}

	if metric.MetricTargetType == "utilization" {
		value := int32(metric.MetricTargetValue)
		ms.Resource.TargetAverageUtilization = &value
	}
	if metric.MetricTargetType == "average_value" {
		if metric.MetricsName == "cpu" {
			ms.Resource.TargetAverageValue = resource.NewMilliQuantity(int64(metric.MetricTargetValue), resource.DecimalSI)
		}
		if metric.MetricsName == "memory" {
			ms.Resource.TargetAverageValue = resource.NewQuantity(int64(metric.MetricTargetValue*1024*1024), resource.BinarySI)
		}
	}

	return ms
}

func newHPA(namespace, kind, name string, labels map[string]string, rule *model.TenantServiceAutoscalerRules, metrics []*model.TenantServiceAutoscalerRuleMetrics) *v2beta1.HorizontalPodAutoscaler {
	hpa := &v2beta1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rule.RuleID,
			Namespace: namespace,
			Labels:    labels,
		},
	}

	spec := v2beta1.HorizontalPodAutoscalerSpec{
		MinReplicas: util.Int32(int32(rule.MinReplicas)),
		MaxReplicas: int32(rule.MaxReplicas),
		ScaleTargetRef: v2beta1.CrossVersionObjectReference{
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

		ms := createResourceMetrics(metric)
		spec.Metrics = append(spec.Metrics, ms)
	}
	hpa.Spec = spec

	return hpa
}
