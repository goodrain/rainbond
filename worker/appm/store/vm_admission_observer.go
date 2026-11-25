package store

import (
	"context"
	"errors"
	"time"

	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// startVMAdmissionObserver consumes actual runtime informer completion events. It
// does not run restore phases or poll a second operation journal.
func (a *appRuntimeStore) startVMAdmissionObserver(stop <-chan struct{}) {
	if a.k8sClient == nil || a.k8sClient.DynamicClient == nil || a.dbmanager == nil || a.dbmanager.DB() == nil {
		return
	}
	observe := func(value interface{}) {
		if tombstone, ok := value.(cache.DeletedFinalStateUnknown); ok {
			value = tombstone.Obj
		}
		if pod, ok := value.(*corev1.Pod); ok && pod.Labels["kubevirt.io"] != "virt-launcher" {
			return
		}
		object, ok := value.(metav1.Object)
		if !ok {
			return
		}
		serviceID := object.GetLabels()["service_id"]
		if serviceID == "" {
			return
		}
		service, err := a.dbmanager.TenantServiceDao().GetServiceByID(serviceID)
		if err != nil || service == nil || !service.IsVM() {
			return
		}
		tenant, err := a.dbmanager.TenantDao().GetTenantByUUID(service.TenantID)
		if err != nil || tenant == nil || tenant.Namespace != object.GetNamespace() {
			return
		}
		scope := vmrestore.Scope{TenantID: service.TenantID, ServiceID: serviceID, Namespace: tenant.Namespace}
		store := vmrestore.NewStore(a.dbmanager.DB())
		resolver := vmrestore.NewResolver(store, a.k8sClient.Clientset, a.k8sClient.DynamicClient)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		// Unknown/not-ready observations retain admission; only exact UID and
		// binding-generation completion can finish it in the shared Store.
		if err := store.ObserveOrdinaryVMCompletion(ctx, resolver, a.k8sClient.DynamicClient, scope); err != nil && !errors.Is(err, vmrestore.ErrRestorePending) {
			logrus.WithError(err).WithField("service_id", serviceID).Warn("observe VM power operation completion")
		}
	}
	var pods cache.SharedIndexInformer
	if a.informers != nil {
		pods = a.informers.Pod
	}
	startVMRuntimeObservers(a.k8sClient.DynamicClient, pods, 30*time.Second, stop, observe)
}

// Resync existing runtime objects so a transient read or concurrent ledger
// update cannot leave a power event pending until the next guest state change.
// VM events also cover completed stops after the VMI and launcher disappear.
func startVMRuntimeObservers(client dynamic.Interface, pods cache.SharedIndexInformer, resync time.Duration, stop <-chan struct{}, observe func(interface{})) {
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(client, resync, "", nil)
	handler := cache.ResourceEventHandlerFuncs{AddFunc: observe, UpdateFunc: func(_, current interface{}) { observe(current) }, DeleteFunc: observe}
	for _, resource := range []string{"virtualmachines", "virtualmachineinstances"} {
		informer := factory.ForResource(schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: resource}).Informer()
		_, _ = informer.AddEventHandler(handler)
	}
	if pods != nil {
		_, _ = pods.AddEventHandlerWithResyncPeriod(handler, resync)
	}
	factory.Start(stop)
}
