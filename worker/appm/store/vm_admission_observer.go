package store

import (
	"context"
	"time"

	"github.com/goodrain/rainbond/pkg/vmrestore"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// startVMAdmissionObserver consumes actual VMI informer completion events. It
// does not run restore phases or poll a second operation journal.
func (a *appRuntimeStore) startVMAdmissionObserver(stop <-chan struct{}) {
	if a.k8sClient == nil || a.k8sClient.DynamicClient == nil || a.dbmanager == nil || a.dbmanager.DB() == nil {
		return
	}
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(a.k8sClient.DynamicClient, 0, "", nil)
	informer := factory.ForResource(schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}).Informer()
	observe := func(value interface{}) {
		if tombstone, ok := value.(cache.DeletedFinalStateUnknown); ok {
			value = tombstone.Obj
		}
		object, ok := value.(*unstructured.Unstructured)
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
		_ = store.ObserveOrdinaryVMCompletion(ctx, resolver, a.k8sClient.DynamicClient, scope)
	}
	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{AddFunc: observe, UpdateFunc: func(_, current interface{}) { observe(current) }, DeleteFunc: observe})
	factory.Start(stop)
}
