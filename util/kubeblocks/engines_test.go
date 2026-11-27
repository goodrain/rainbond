package kubeblocks

import "testing"

func TestAdditionalEngineServiceSelectors(t *testing.T) {
	for _, tc := range []struct {
		component string
		primary   bool
	}{
		{"mysql", true}, {"postgresql", true}, {"redis", true}, {"rabbitmq", false},
		{"mongodb", true}, {"kafka-combine", false}, {"mdit", false},
		{"minio", false}, {"clickhouse", false},
	} {
		t.Run(tc.component, func(t *testing.T) {
			s := GenerateKubeBlocksSelector("team-orders-a123-" + tc.component)
			if s["app.kubernetes.io/instance"] != "team-orders-a123" || s["apps.kubeblocks.io/component-name"] != tc.component {
				t.Fatalf("selector addresses the wrong cluster or component: %#v", s)
			}
			if (s["kubeblocks.io/role"] == "primary") != tc.primary {
				t.Fatalf("unexpected primary role requirement: %#v", s)
			}
		})
	}
}
