package discover

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/discover/config"
)

type k8sDiscover struct {
	ctx       context.Context
	cancel    context.CancelFunc
	lock      sync.Mutex
	clientset kubernetes.Interface
	cfg       *option.Conf
	projects  map[string]CallbackUpdate
}

// NewK8sDiscover creates a new Discover
func NewK8sDiscover(ctx context.Context, clientset kubernetes.Interface, cfg *option.Conf) Discover {
	ctx, cancel := context.WithCancel(ctx)
	return &k8sDiscover{
		ctx:       ctx,
		cancel:    cancel,
		clientset: clientset,
		cfg:       cfg,
		projects:  make(map[string]CallbackUpdate),
	}
}

func (k *k8sDiscover) Stop() {
	k.cancel()
}

func (k *k8sDiscover) AddProject(name string, callback Callback) {
	k.lock.Lock()
	defer k.lock.Unlock()

	if _, ok := k.projects[name]; !ok {
		cal := &defaultCallBackUpdate{
			callback:  callback,
			endpoints: make(map[string]*config.Endpoint),
		}
		k.projects[name] = cal
		go k.discover(name, cal)
	}
}

func (k *k8sDiscover) AddUpdateProject(name string, callback CallbackUpdate) {
	k.lock.Lock()
	defer k.lock.Unlock()

	if _, ok := k.projects[name]; !ok {
		k.projects[name] = callback
		go k.discover(name, callback)
	}
}

func (k *k8sDiscover) discover(name string, callback CallbackUpdate) {
	endpoints := k.list(name)
	if len(endpoints) > 0 {
		callback.UpdateEndpoints(config.SYNC, endpoints...)
	}

	sharedInformer := informers.NewSharedInformerFactoryWithOptions(
		k.clientset,
		10*time.Second,
		informers.WithNamespace(k.cfg.RbdNamespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = "name=" + name
		}),
	)

	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			ep := endpointForPod(pod)
			callback.UpdateEndpoints(config.SYNC, ep)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			ep := endpointForPod(pod)
			ep.Mode = 2
			callback.UpdateEndpoints(config.DELETE, ep)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldPod := old.(*corev1.Pod)
			curPod := cur.(*corev1.Pod)

			if reflect.DeepEqual(oldPod, curPod) {
				return
			}

			ep := endpointForPod(curPod)
			if !isPodReady(curPod) {
				logrus.Infof("unready pod(%s%s) received, delete endpoint based on the pod", curPod.Name, curPod.Namespace)
				ep.Mode = 2
				callback.UpdateEndpoints(config.DELETE, ep)
				return
			}
			callback.UpdateEndpoints(config.SYNC, ep)
		},
	}

	infomer := sharedInformer.Core().V1().Pods().Informer()
	infomer.AddEventHandler(eventHandler)

	// start
	go infomer.Run(k.ctx.Done())

	if !cache.WaitForCacheSync(k.ctx.Done(), infomer.HasSynced) {
		k.rewatchWithErr(name, callback, errors.New("timeout wait for cache sync"))
	}
}

func (k *k8sDiscover) removeProject(name string) {
	k.lock.Lock()
	defer k.lock.Unlock()
	if _, ok := k.projects[name]; ok {
		delete(k.projects, name)
	}
}

func (k *k8sDiscover) rewatchWithErr(name string, callback CallbackUpdate, err error) {
	logrus.Debugf("name: %s; monitor discover get watch error: %s, remove this watch target first, and then sleep 10 sec, we will re-watch it", name, err.Error())
	callback.Error(err)
	k.removeProject(name)
	time.Sleep(10 * time.Second)
	k.AddUpdateProject(name, callback)
}

func (k *k8sDiscover) list(name string) []*config.Endpoint {
	podList, err := k.clientset.CoreV1().Pods(k.cfg.RbdNamespace).List(metav1.ListOptions{
		LabelSelector: "name=" + name,
	})
	if err != nil {
		logrus.Warningf("list pods for %s: %v", name, err)
		return nil
	}

	var endpoints []*config.Endpoint
	var notReadyEp *config.Endpoint
	for _, pod := range podList.Items {
		ep := endpointForPod(&pod)
		if isPodReady(&pod) {
			endpoints = append(endpoints, ep)
			continue
		}
		if notReadyEp == nil {
			notReadyEp = endpointForPod(&pod)
		}
	}

	// If there are no ready endpoints, a not ready endpoint is used
	if len(endpoints) == 0 && notReadyEp != nil {
		endpoints = append(endpoints, notReadyEp)
	}

	return endpoints
}

func endpointForPod(pod *corev1.Pod) *config.Endpoint {
	return &config.Endpoint{
		Name: pod.Name,
		URL:  pod.Status.PodIP,
	}
}

func isPodReady(pod *corev1.Pod) bool {
	if pod.ObjectMeta.DeletionTimestamp != nil {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
