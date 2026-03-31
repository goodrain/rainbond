package conversion

import (
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/stretchr/testify/assert"
)

// capability_id: rainbond.worker.appm.autoscaler.build-hpa-spec
func TestCreateMetricSpec(t *testing.T) {
	metric := &model.TenantServiceAutoscalerRuleMetrics{
		MetricsType:       "resource_metrics",
		MetricsName:       "memory",
		MetricTargetType:  "average_value",
		MetricTargetValue: 60,
	}

	metricSpec := createResourceMetrics(metric)
	assert.Equal(t, "Resource", string(metricSpec.Type))
	if assert.NotNil(t, metricSpec.Resource) {
		assert.Equal(t, "memory", string(metricSpec.Resource.Name))
		assert.Equal(t, "AverageValue", string(metricSpec.Resource.Target.Type))
		if assert.NotNil(t, metricSpec.Resource.Target.AverageValue) {
			assert.Equal(t, "60Mi", metricSpec.Resource.Target.AverageValue.String())
		}
	}
}

// capability_id: rainbond.worker.appm.autoscaler.build-hpa-spec
func TestNewHPA(t *testing.T) {
	t.Skip("integration test depends on local kubeconfig and live cluster")
	rule := &model.TenantServiceAutoscalerRules{
		RuleID:      "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		ServiceID:   "45197f4936cf45efa2ac4831ce42025a",
		MinReplicas: 1,
		MaxReplicas: 10,
	}
	metrics := []*model.TenantServiceAutoscalerRuleMetrics{
		{
			MetricsType:       "resource_metrics",
			MetricsName:       "cpu",
			MetricTargetType:  "utilization",
			MetricTargetValue: 50,
		},
		{
			MetricsType:       "resource_metrics",
			MetricsName:       "memory",
			MetricTargetType:  "average_value",
			MetricTargetValue: 60,
		},
	}
	namespace := "bab18e6b1c8640979b91f8dfdd211226"
	kind := "Deployment"
	name := "45197f4936cf45efa2ac4831ce42025a-deployment-6d84f798b4-tmvfc"

	hpa := newHPA(namespace, kind, name, nil, rule, metrics)

	if assert.NotNil(t, hpa) {
		assert.Equal(t, rule.RuleID, hpa.Name)
		assert.Equal(t, namespace, hpa.Namespace)
		assert.Equal(t, kind, hpa.Spec.ScaleTargetRef.Kind)
		assert.Equal(t, name, hpa.Spec.ScaleTargetRef.Name)
		if assert.NotNil(t, hpa.Spec.MinReplicas) {
			assert.Equal(t, int32(1), *hpa.Spec.MinReplicas)
		}
		assert.Equal(t, int32(10), hpa.Spec.MaxReplicas)
		if assert.Len(t, hpa.Spec.Metrics, 2) {
			assert.Equal(t, "cpu", string(hpa.Spec.Metrics[0].Resource.Name))
			assert.Equal(t, "Utilization", string(hpa.Spec.Metrics[0].Resource.Target.Type))
			assert.Equal(t, "memory", string(hpa.Spec.Metrics[1].Resource.Name))
			assert.Equal(t, "AverageValue", string(hpa.Spec.Metrics[1].Resource.Target.Type))
		}
	}
}
