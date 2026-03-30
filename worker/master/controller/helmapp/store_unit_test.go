package helmapp

import (
	"testing"

	rainbondv1alpha1 "github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	listers "github.com/goodrain/rainbond/pkg/generated/listers/rainbond/v1alpha1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// capability_id: rainbond.worker.helmapp.store-fetch
func TestStoreGetHelmApp(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	obj := &rainbondv1alpha1.HelmApp{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
	}
	if err := indexer.Add(obj); err != nil {
		t.Fatal(err)
	}

	s := &store{lister: listers.NewHelmAppLister(indexer)}
	got, err := s.GetHelmApp("default", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "demo" || got.Namespace != "default" {
		t.Fatalf("unexpected helm app: %+v", got)
	}
}

// capability_id: rainbond.worker.helmapp.finalizer-stop
func TestFinalizerStop(t *testing.T) {
	queue := workqueue.New()
	finalizer := &Finalizer{
		queue: queue,
		log:   logrus.WithField("test", "finalizer"),
	}

	finalizer.Stop()
	if !queue.ShuttingDown() {
		t.Fatal("expected finalizer queue to be shutting down")
	}
}
