package helmapp

import (
	"fmt"
	"time"

	rainbondv1alpha1 "github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/goodrain/rainbond/pkg/generated/informers/externalversions"
	"github.com/goodrain/rainbond/pkg/generated/listers/rainbond/v1alpha1"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Storer -
type Storer interface {
	Run(stopCh <-chan struct{})
	GetHelmApp(ns, name string) (*rainbondv1alpha1.HelmApp, error)
}

type store struct {
	informer cache.SharedIndexInformer
	lister   v1alpha1.HelmAppLister
}

func NewStorer(clientset versioned.Interface,
	resyncPeriod time.Duration,
	workqueue workqueue.Interface,
	finalizerQueue workqueue.Interface) Storer {
	// create informers factory, enable and assign required informers
	sharedInformer := externalversions.NewSharedInformerFactoryWithOptions(clientset, resyncPeriod,
		externalversions.WithNamespace(corev1.NamespaceAll))

	lister := sharedInformer.Rainbond().V1alpha1().HelmApps().Lister()

	informer := sharedInformer.Rainbond().V1alpha1().HelmApps().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			helmApp := obj.(*rainbondv1alpha1.HelmApp)
			workqueue.Add(k8sutil.ObjKey(helmApp))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			helmApp := newObj.(*rainbondv1alpha1.HelmApp)
			workqueue.Add(k8sutil.ObjKey(helmApp))
		},
		DeleteFunc: func(obj interface{}) {
			// Two purposes of using finalizerQueue
			// 1. non-block DeleteFunc
			// 2. retry if the finalizer is failed
			finalizerQueue.Add(obj)
		},
	})

	return &store{
		informer: informer,
		lister:   lister,
	}
}

func (i *store) Run(stopCh <-chan struct{}) {
	go i.informer.Run(stopCh)

	// wait for all involved caches to be synced before processing items
	// from the queue
	if !cache.WaitForCacheSync(stopCh,
		i.informer.HasSynced,
	) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
	}

	// in big clusters, deltas can keep arriving even after HasSynced
	// functions have returned 'true'
	time.Sleep(1 * time.Second)
}

func (i *store) GetHelmApp(ns, name string) (*rainbondv1alpha1.HelmApp, error) {
	return i.lister.HelmApps(ns).Get(name)
}
