package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/goodrain/rainbond/api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// capability_id: rainbond.overview.deployment-replica-status
func TestRainbondComponentDeploymentReplicaStatus(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*componentReplicaFixture)
		want   map[string]int
	}{
		{name: "completed rollout history", want: map[string]int{"ready": 1, "total": 1}},
		{name: "failed rollout history", mutate: func(f *componentReplicaFixture) { f.pods[1].Status.Phase = corev1.PodFailed }, want: map[string]int{"ready": 1, "total": 1}},
		{name: "real desired two", mutate: func(f *componentReplicaFixture) { *f.deployment.Spec.Replicas = 2 }, want: map[string]int{"ready": 1, "total": 2}},
		{name: "pending replica stays unhealthy", mutate: func(f *componentReplicaFixture) {
			p := f.pods[0].DeepCopy()
			p.Name = "pending"
			p.UID = "pending"
			p.Status = corev1.PodStatus{Phase: corev1.PodPending}
			f.pods = append(f.pods, *p)
		}, want: map[string]int{"ready": 1, "total": 2}},
		{name: "running unready stays unhealthy", mutate: func(f *componentReplicaFixture) { f.pods[0].Status.Conditions[0].Status = corev1.ConditionFalse }, want: map[string]int{"ready": 0, "total": 1}},
		{name: "only terminal keeps desired", mutate: func(f *componentReplicaFixture) { f.pods = f.pods[1:] }, want: map[string]int{"ready": 0, "total": 1}},
		{name: "missing pod owner", mutate: func(f *componentReplicaFixture) { f.pods[0].OwnerReferences = nil }},
		{name: "pod owner is not controller", mutate: func(f *componentReplicaFixture) {
			f.pods[0].OwnerReferences[0].Controller = componentReplicaBool(false)
		}},
		{name: "pod owner wrong API", mutate: func(f *componentReplicaFixture) { f.pods[0].OwnerReferences[0].APIVersion = "other/v1" }},
		{name: "pod owner missing UID", mutate: func(f *componentReplicaFixture) { f.pods[0].OwnerReferences[0].UID = "" }},
		{name: "mixed Job and RS pods", mutate: func(f *componentReplicaFixture) {
			f.pods[0].OwnerReferences = []metav1.OwnerReference{componentReplicaOwner("Job", "batch/v1", "job", "job")}
		}},
		{name: "ambiguous pod controllers", mutate: func(f *componentReplicaFixture) {
			f.pods[0].OwnerReferences = append(f.pods[0].OwnerReferences, f.pods[0].OwnerReferences[0])
		}},
		{name: "pod RS UID mismatch", mutate: func(f *componentReplicaFixture) { f.pods[0].OwnerReferences[0].UID = "another-rs" }},
		{name: "pod wrong namespace", mutate: func(f *componentReplicaFixture) { f.pods[0].Namespace = "other" }},
		{name: "RS wrong namespace", mutate: func(f *componentReplicaFixture) { f.replicaSets["new"].Namespace = "other" }},
		{name: "RS wrong name", mutate: func(f *componentReplicaFixture) { f.replicaSets["new"].Name = "other" }},
		{name: "missing RS deployment owner", mutate: func(f *componentReplicaFixture) { f.replicaSets["new"].OwnerReferences = nil }},
		{name: "RS deployment owner is not controller", mutate: func(f *componentReplicaFixture) {
			f.replicaSets["new"].OwnerReferences[0].Controller = componentReplicaBool(false)
		}},
		{name: "RS owner not deployment", mutate: func(f *componentReplicaFixture) { f.replicaSets["new"].OwnerReferences[0].Kind = "StatefulSet" }},
		{name: "RS owner wrong API", mutate: func(f *componentReplicaFixture) { f.replicaSets["new"].OwnerReferences[0].APIVersion = "other/v1" }},
		{name: "ambiguous RS controllers", mutate: func(f *componentReplicaFixture) {
			f.replicaSets["new"].OwnerReferences = append(f.replicaSets["new"].OwnerReferences, f.replicaSets["new"].OwnerReferences[0])
		}},
		{name: "deployment UID mismatch", mutate: func(f *componentReplicaFixture) { f.deployment.UID = "different" }},
		{name: "mixed deployment owners", mutate: func(f *componentReplicaFixture) { f.replicaSets["old"].OwnerReferences[0].UID = "different" }},
		{name: "mixed deployment names", mutate: func(f *componentReplicaFixture) { f.replicaSets["old"].OwnerReferences[0].Name = "different" }},
		{name: "deployment wrong namespace", mutate: func(f *componentReplicaFixture) { f.deployment.Namespace = "other" }},
		{name: "deployment wrong name", mutate: func(f *componentReplicaFixture) { f.deployment.Name = "other" }},
		{name: "deleting deployment", mutate: func(f *componentReplicaFixture) { now := metav1.Now(); f.deployment.DeletionTimestamp = &now }},
		{name: "unobserved generation", mutate: func(f *componentReplicaFixture) { f.deployment.Status.ObservedGeneration = 1 }},
		{name: "no desired replicas", mutate: func(f *componentReplicaFixture) { f.deployment.Spec.Replicas = nil }},
		{name: "scaled to zero", mutate: func(f *componentReplicaFixture) { *f.deployment.Spec.Replicas = 0 }},
		{name: "RS lookup failure", mutate: func(f *componentReplicaFixture) { f.failPath = "/apis/apps/v1/namespaces/rbd-system/replicasets/new" }},
		{name: "deployment lookup failure", mutate: func(f *componentReplicaFixture) {
			f.failPath = "/apis/apps/v1/namespaces/rbd-system/deployments/gateway"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newComponentReplicaFixture()
			if tt.mutate != nil {
				tt.mutate(f)
			}
			action, closeServer := f.action(t)
			defer closeServer()
			got, err := action.ListRainbondComponents(context.Background())
			require.NoError(t, err)
			gateway := componentReplicaNamed(t, got, "rbd-gateway")
			assert.Len(t, gateway.Pods, len(f.pods))
			assert.Equal(t, len(f.pods), gateway.AllPods)
			ready := 0
			for _, p := range f.pods {
				if podReadyForOverview(p) {
					ready++
				}
			}
			assert.Equal(t, ready, gateway.RunPods)
			b, err := json.Marshal(gateway)
			require.NoError(t, err)
			var payload struct {
				ReplicaStatus map[string]int `json:"replica_status"`
			}
			require.NoError(t, json.Unmarshal(b, &payload))
			assert.Equal(t, tt.want, payload.ReplicaStatus)
			if tt.want == nil {
				assert.NotContains(t, string(b), `"replica_status"`)
			}
		})
	}
}

func TestRainbondComponentReplicaStatusQueriesAreDemandDriven(t *testing.T) {
	for _, kind := range []string{"ordinary", "job", "missing-terminal-owner"} {
		t.Run(kind, func(t *testing.T) {
			f := newComponentReplicaFixture()
			switch kind {
			case "ordinary":
				f.pods = f.pods[:1]
			case "job":
				for i := range f.pods {
					f.pods[i].OwnerReferences = []metav1.OwnerReference{componentReplicaOwner("Job", "batch/v1", "job", "job")}
				}
			case "missing-terminal-owner":
				f.pods[1].OwnerReferences = nil
			}
			action, closeServer := f.action(t)
			defer closeServer()
			got, err := action.ListRainbondComponents(context.Background())
			require.NoError(t, err)
			assert.Len(t, componentReplicaNamed(t, got, "rbd-gateway").Pods, len(f.pods))
			assert.Zero(t, f.ownerCalls())
		})
	}
}

func TestRainbondComponentReplicaStatusRequestCacheAndLegacyOrdering(t *testing.T) {
	f := newComponentReplicaFixture()
	for _, p := range append([]corev1.Pod(nil), f.pods...) {
		p.Name += "-second"
		p.UID += "-second"
		p.Labels = map[string]string{"name": "aaa"}
		f.pods = append(f.pods, p)
	}
	f.operatorPods = []corev1.Pod{f.pods[0]}
	action, closeServer := f.action(t)
	defer closeServer()
	for i := 0; i < 2; i++ {
		got, err := action.ListRainbondComponents(context.Background())
		require.NoError(t, err)
		require.Len(t, got, 3)
		assert.Equal(t, []string{"aaa", "rainbond-operator", "rbd-gateway"}, []string{got[0].Name, got[1].Name, got[2].Name})
		assert.Equal(t, 2, componentReplicaNamed(t, got, "rbd-gateway").AllPods)
		assert.Empty(t, componentReplicaNamed(t, got, "rainbond-operator").Pods)
		assert.Equal(t, 3*(i+1), f.ownerCalls(), "RS and Deployment gets are cached only within a request")
	}
}

func TestRainbondComponentReplicaStatusRespectsRequestDeadline(t *testing.T) {
	for _, tt := range []struct {
		name           string
		parentTimeout  time.Duration
		multipleGroups bool
		maxDuration    time.Duration
		wantOwnerCalls int
	}{
		{name: "slow owner preserves raw response", maxDuration: time.Second, wantOwnerCalls: 3},
		{name: "all candidates share one optional budget", multipleGroups: true, maxDuration: time.Second, wantOwnerCalls: 1},
		{name: "shorter parent deadline wins", parentTimeout: 50 * time.Millisecond, maxDuration: 400 * time.Millisecond, wantOwnerCalls: 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newComponentReplicaFixture()
			f.blockPath = "/apis/apps/v1/namespaces/rbd-system/deployments/gateway"
			if tt.multipleGroups {
				f.pods[0].Labels["name"] = "aaa"
				f.pods[0].Status.Phase = corev1.PodSucceeded
				f.pods[1].Labels["name"] = "bbb"
				f.blockPaths = map[string]bool{
					"/apis/apps/v1/namespaces/rbd-system/replicasets/new": true,
					"/apis/apps/v1/namespaces/rbd-system/replicasets/old": true,
				}
			}
			action, closeServer := f.action(t)
			defer closeServer()
			ctx := context.Background()
			if tt.parentTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.parentTimeout)
				defer cancel()
			}
			started := time.Now()
			got, err := action.ListRainbondComponents(ctx)
			require.NoError(t, err)
			assert.Less(t, time.Since(started), tt.maxDuration, "optional ownership reads must not consume the main request budget")
			assert.Equal(t, tt.wantOwnerCalls, f.ownerCalls())
			for _, component := range got {
				var wantPods []corev1.Pod
				wantReady := 0
				for _, pod := range f.pods {
					if pod.Labels["name"] == component.Name {
						wantPods = append(wantPods, pod)
						if podReadyForOverview(pod) {
							wantReady++
						}
					}
				}
				if len(wantPods) > 0 {
					assert.Equal(t, wantPods, component.Pods, "raw Pod inventory must survive timeout")
				}
				assert.Equal(t, len(wantPods), component.AllPods)
				assert.Equal(t, wantReady, component.RunPods)
				b, err := json.Marshal(component)
				require.NoError(t, err)
				assert.NotContains(t, string(b), `"replica_status"`)
			}
		})
	}
}

func TestRainbondComponentReplicaStatusCachesFailedLookups(t *testing.T) {
	for _, resource := range []string{"replicasets/new", "deployments/gateway"} {
		t.Run(resource, func(t *testing.T) {
			f := newComponentReplicaFixture()
			f.failPath = "/apis/apps/v1/namespaces/rbd-system/" + resource
			for _, pod := range append([]corev1.Pod(nil), f.pods...) {
				pod.Name += "-second"
				pod.UID += "-second"
				pod.Labels = map[string]string{"name": "second"}
				f.pods = append(f.pods, pod)
			}
			action, closeServer := f.action(t)
			defer closeServer()
			got, err := action.ListRainbondComponents(context.Background())
			require.NoError(t, err)
			for _, component := range got {
				b, err := json.Marshal(component)
				require.NoError(t, err)
				assert.NotContains(t, string(b), `"replica_status"`)
			}
			f.mu.Lock()
			calls := f.calls[f.failPath]
			f.mu.Unlock()
			assert.Equal(t, 1, calls, "failed lookups must also be cached within the request")
		})
	}
}

type componentReplicaFixture struct {
	pods, operatorPods  []corev1.Pod
	replicaSets         map[string]*appsv1.ReplicaSet
	deployment          *appsv1.Deployment
	failPath, blockPath string
	blockPaths          map[string]bool
	mu                  sync.Mutex
	calls               map[string]int
}

func newComponentReplicaFixture() *componentReplicaFixture {
	replicas := int32(1)
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gateway", Namespace: "rbd-system", UID: "deployment", Generation: 2}, Spec: appsv1.DeploymentSpec{Replicas: &replicas}, Status: appsv1.DeploymentStatus{ObservedGeneration: 2}}
	f := &componentReplicaFixture{deployment: deployment, replicaSets: map[string]*appsv1.ReplicaSet{}, calls: map[string]int{}}
	for _, name := range []string{"new", "old"} {
		f.replicaSets[name] = &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "rbd-system", UID: types.UID(name), OwnerReferences: []metav1.OwnerReference{componentReplicaOwner("Deployment", "apps/v1", "gateway", "deployment")}}}
	}
	f.replicaSets["new"].Spec.Replicas = &replicas
	zero := int32(0)
	f.replicaSets["old"].Spec.Replicas = &zero
	for _, name := range []string{"new", "old"} {
		f.pods = append(f.pods, corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "rbd-system", UID: types.UID("pod-" + name), Labels: map[string]string{"name": "rbd-gateway"}, OwnerReferences: []metav1.OwnerReference{componentReplicaOwner("ReplicaSet", "apps/v1", name, types.UID(name))}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}})
	}
	f.pods[1].Status.Phase = corev1.PodSucceeded
	return f
}

func (f *componentReplicaFixture) action(t *testing.T) (*clusterAction, func()) {
	t.Helper()
	t.Setenv("RBD_NAMESPACE", "rbd-system")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.calls[r.URL.Path]++
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == f.blockPath || f.blockPaths[r.URL.Path] {
			<-r.Context().Done()
			return
		}
		if r.URL.Path == f.failPath {
			http.Error(w, "lookup failed", http.StatusForbidden)
			return
		}
		var data interface{}
		switch r.URL.Path {
		case "/api/v1/namespaces/rbd-system/pods":
			pods := f.pods
			if r.URL.Query().Get("labelSelector") == "name=rainbond-operator" {
				pods = f.operatorPods
			}
			data = &corev1.PodList{Items: pods}
		case "/apis/apps/v1/namespaces/rbd-system/replicasets/new":
			data = f.replicaSets["new"]
		case "/apis/apps/v1/namespaces/rbd-system/replicasets/old":
			data = f.replicaSets["old"]
		case "/apis/apps/v1/namespaces/rbd-system/deployments/gateway":
			data = f.deployment
		default:
			http.NotFound(w, r)
			return
		}
		if err := json.NewEncoder(w).Encode(data); err != nil {
			t.Errorf("encode fixture: %v", err)
		}
	}))
	cli, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	return &clusterAction{clientset: cli}, server.Close
}

func (f *componentReplicaFixture) ownerCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	sum := 0
	for path, n := range f.calls {
		if path != "/api/v1/namespaces/rbd-system/pods" {
			sum += n
		}
	}
	return sum
}
func componentReplicaOwner(kind, apiVersion, name string, uid types.UID) metav1.OwnerReference {
	return metav1.OwnerReference{Kind: kind, APIVersion: apiVersion, Name: name, UID: uid, Controller: componentReplicaBool(true)}
}
func componentReplicaBool(v bool) *bool { return &v }
func componentReplicaNamed(t *testing.T, components []*model.RainbondComponent, name string) *model.RainbondComponent {
	t.Helper()
	for _, component := range components {
		if component.Name == name {
			return component
		}
	}
	t.Fatalf("component %s missing", name)
	return nil
}
