package store

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/goodrain/rainbond/pkg/component/k8s"
	runtimev1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

func TestOperatorManagedViewsReadPodsFromInformerLister(t *testing.T) {
	var livePodLists int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&livePodLists, 1)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(&corev1.PodList{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "PodList"},
		}))
	}))
	defer server.Close()
	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, pod := range []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "operator-demo-abc",
				Namespace: "tenant-ns",
				Labels:    map[string]string{"app": "operator-demo"},
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "Deployment", Name: "operator-demo-abc",
				}},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "demo:v1"}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "operator-db-0", Namespace: "tenant-ns", Labels: map[string]string{"app": "operator-db"}},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}},
		},
	} {
		require.NoError(t, indexer.Add(pod))
	}

	store := &appRuntimeStore{
		k8sClient: &k8s.Component{Clientset: clientset},
		listers:   &Lister{Pod: corelisters.NewPodLister(indexer)},
	}
	managed := &runtimev1.OperatorManaged{AppID: "app-id"}
	managed.SetService(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "operator-service", Namespace: "tenant-ns"},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "operator-demo"}},
	})
	managed.SetDeployment(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "operator-demo", Namespace: "tenant-ns"}})
	managed.SetStatefulSet(&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "operator-db", Namespace: "tenant-ns"}})

	services := store.HandleOperatorManagedService(managed)
	deployments := store.HandleOperatorManagedDeployment(managed)
	statefulSets := store.HandleOperatorManagedStatefulSet(managed)

	require.Len(t, services, 1)
	assert.Equal(t, []string{"operator-demo"}, services[0].Relation)
	require.Len(t, deployments, 1)
	assert.Len(t, deployments[0].Pods, 1)
	require.Len(t, statefulSets, 1)
	assert.Len(t, statefulSets[0].Pods, 1)
	assert.Zero(t, atomic.LoadInt32(&livePodLists), "operator-managed views must not issue live pod list requests")
}
