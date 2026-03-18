package handler

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

func TestLabelsMatchSelector(t *testing.T) {
	assert.True(t, labelsMatchSelector(
		map[string]string{"app": "demo"},
		map[string]string{"app": "demo", "version": "v1"},
	))
	assert.False(t, labelsMatchSelector(
		map[string]string{"app": "demo", "component": "web"},
		map[string]string{"app": "demo"},
	))
	assert.False(t, labelsMatchSelector(
		map[string]string{"app": "demo"},
		map[string]string{"app": "other"},
	))
	assert.False(t, labelsMatchSelector(
		nil,
		map[string]string{"app": "demo"},
	))
}

func TestCollectIngressServiceNames(t *testing.T) {
	ingress := networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{Name: "default-svc"},
			},
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{Name: "api-svc"},
									},
								},
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{Name: "web-svc"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	assert.ElementsMatch(t, []string{"default-svc", "api-svc", "web-svc"}, collectIngressServiceNames(ingress))
}

func TestToResourceEventInfo(t *testing.T) {
	lastTime := metav1.NewTime(time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC))
	event := corev1.Event{
		Type:           "Warning",
		Reason:         "FailedScheduling",
		Message:        "0/3 nodes are available",
		Count:          2,
		LastTimestamp:  lastTime,
		FirstTimestamp: lastTime,
	}

	info := toResourceEventInfo(event)
	assert.Equal(t, "Warning", info.Type)
	assert.Equal(t, "FailedScheduling", info.Reason)
	assert.Equal(t, "0/3 nodes are available", info.Message)
	assert.Equal(t, int32(2), info.Count)
	assert.Equal(t, lastTime.String(), info.LastTimestamp)
}
