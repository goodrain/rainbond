/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/cmd/worker/option"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	v1 "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	storagebeta "k8s.io/api/storage/v1beta1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	ref "k8s.io/client-go/tools/reference"
	"k8s.io/client-go/util/workqueue"

	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/worker/master/volumes/provider/lib/controller/metrics"
	"github.com/goodrain/rainbond/worker/master/volumes/provider/lib/util"
)

// annClass annotation represents the storage class associated with a resource:
// - in PersistentVolumeClaim it represents required class to match.
//   Only PersistentVolumes with the same class (i.e. annotation with the same
//   value) can be bound to the claim. In case no such volume exists, the
//   controller will provision a new one using StorageClass instance with
//   the same name as the annotation value.
// - in PersistentVolume it represents storage class to which the persistent
//   volume belongs.
const annClass = "volume.beta.kubernetes.io/storage-class"

// This annotation is added to a PV that has been dynamically provisioned by
// Kubernetes. Its value is name of volume plugin that created the volume.
// It serves both user (to show where a PV comes from) and Kubernetes (to
// recognize dynamically provisioned PVs in its decisions).
const annDynamicallyProvisioned = "pv.kubernetes.io/provisioned-by"

const annStorageProvisioner = "volume.beta.kubernetes.io/storage-provisioner"

// This annotation is added to a PVC that has been triggered by scheduler to
// be dynamically provisioned. Its value is the name of the selected node.
const annSelectedNode = "volume.kubernetes.io/selected-node"

// ProvisionController is a controller that provisions PersistentVolumes for
// PersistentVolumeClaims.
type ProvisionController struct {
	client kubernetes.Interface
	cfg    *option.Config

	// The provisioner the controller will use to provision and delete volumes.
	// Presumably this implementer of Provisioner carries its own
	// volume-specific options and such that it needs in order to provision
	// volumes.
	provisioners map[string]Provisioner

	// Kubernetes cluster server version:
	// * 1.4: storage classes introduced as beta. Technically out-of-tree dynamic
	// provisioning is not officially supported, though it works
	// * 1.5: storage classes stay in beta. Out-of-tree dynamic provisioning is
	// officially supported
	// * 1.6: storage classes enter GA
	kubeVersion *utilversion.Version

	claimInformer  cache.SharedInformer
	claims         cache.Store
	volumeInformer cache.SharedInformer
	volumes        cache.Store
	classInformer  cache.SharedInformer
	classes        cache.Store

	// To determine if the informer is internal or external
	customClaimInformer, customVolumeInformer, customClassInformer bool

	claimQueue  workqueue.RateLimitingInterface
	volumeQueue workqueue.RateLimitingInterface

	// Identity of this controller, generated at creation time and not persisted
	// across restarts. Useful only for debugging, for seeing the source of
	// events. controller.provisioner may have its own, different notion of
	// identity which may/may not persist across restarts
	id            string
	component     string
	eventRecorder record.EventRecorder

	resyncPeriod time.Duration

	exponentialBackOffOnError bool
	threadiness               int

	createProvisionedPVRetryCount int
	createProvisionedPVInterval   time.Duration

	failedProvisionThreshold, failedDeleteThreshold int

	// The port for metrics server to serve on.
	metricsPort int32
	// The IP address for metrics server to serve on.
	metricsAddress string
	// The path of metrics endpoint path.
	metricsPath string

	// Whether to do kubernetes leader election at all. It should basically
	// always be done when possible to avoid duplicate Provision attempts.
	leaderElection          bool
	leaderElectionNamespace string
	// Parameters of leaderelection.LeaderElectionConfig.
	leaseDuration, renewDeadline, retryPeriod time.Duration

	hasRun     bool
	hasRunLock *sync.Mutex
}

const (
	// DefaultResyncPeriod is used when option function ResyncPeriod is omitted
	DefaultResyncPeriod = 15 * time.Minute
	// DefaultThreadiness is used when option function Threadiness is omitted
	DefaultThreadiness = 4
	// DefaultExponentialBackOffOnError is used when option function ExponentialBackOffOnError is omitted
	DefaultExponentialBackOffOnError = true
	// DefaultCreateProvisionedPVRetryCount is used when option function CreateProvisionedPVRetryCount is omitted
	DefaultCreateProvisionedPVRetryCount = 5
	// DefaultCreateProvisionedPVInterval is used when option function CreateProvisionedPVInterval is omitted
	DefaultCreateProvisionedPVInterval = 10 * time.Second
	// DefaultFailedProvisionThreshold is used when option function FailedProvisionThreshold is omitted
	DefaultFailedProvisionThreshold = 15
	// DefaultFailedDeleteThreshold is used when option function FailedDeleteThreshold is omitted
	DefaultFailedDeleteThreshold = 15
	// DefaultLeaderElection is used when option function LeaderElection is omitted
	DefaultLeaderElection = true
	// DefaultLeaseDuration is used when option function LeaseDuration is omitted
	DefaultLeaseDuration = 15 * time.Second
	// DefaultRenewDeadline is used when option function RenewDeadline is omitted
	DefaultRenewDeadline = 10 * time.Second
	// DefaultRetryPeriod is used when option function RetryPeriod is omitted
	DefaultRetryPeriod = 2 * time.Second
	// DefaultMetricsPort is used when option function MetricsPort is omitted
	DefaultMetricsPort = 0
	// DefaultMetricsAddress is used when option function MetricsAddress is omitted
	DefaultMetricsAddress = "0.0.0.0"
	// DefaultMetricsPath is used when option function MetricsPath is omitted
	DefaultMetricsPath = "/metrics"
)

var errRuntime = fmt.Errorf("cannot call option functions after controller has Run")

// ResyncPeriod is how often the controller relists PVCs, PVs, & storage
// classes. OnUpdate will be called even if nothing has changed, meaning failed
// operations may be retried on a PVC/PV every resyncPeriod regardless of
// whether it changed. Defaults to 15 minutes.
func ResyncPeriod(resyncPeriod time.Duration) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.resyncPeriod = resyncPeriod
		return nil
	}
}

// Threadiness is the number of claim and volume workers each to launch.
// Defaults to 4.
func Threadiness(threadiness int) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.threadiness = threadiness
		return nil
	}
}

// ExponentialBackOffOnError determines whether to exponentially back off from
// failures of Provision and Delete. Defaults to true.
func ExponentialBackOffOnError(exponentialBackOffOnError bool) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.exponentialBackOffOnError = exponentialBackOffOnError
		return nil
	}
}

// CreateProvisionedPVRetryCount is the number of retries when we create a PV
// object for a provisioned volume. Defaults to 5.
func CreateProvisionedPVRetryCount(createProvisionedPVRetryCount int) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.createProvisionedPVRetryCount = createProvisionedPVRetryCount
		return nil
	}
}

// CreateProvisionedPVInterval is the interval between retries when we create a
// PV object for a provisioned volume. Defaults to 10 seconds.
func CreateProvisionedPVInterval(createProvisionedPVInterval time.Duration) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.createProvisionedPVInterval = createProvisionedPVInterval
		return nil
	}
}

// FailedProvisionThreshold is the threshold for max number of retries on
// failures of Provision. Defaults to 15.
func FailedProvisionThreshold(failedProvisionThreshold int) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.failedProvisionThreshold = failedProvisionThreshold
		return nil
	}
}

// FailedDeleteThreshold is the threshold for max number of retries on failures
// of Delete. Defaults to 15.
func FailedDeleteThreshold(failedDeleteThreshold int) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.failedDeleteThreshold = failedDeleteThreshold
		return nil
	}
}

// LeaderElection determines whether to enable leader election or not. Defaults
// to true.
func LeaderElection(leaderElection bool) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.leaderElection = leaderElection
		return nil
	}
}

// LeaderElectionNamespace is the kubernetes namespace in which to create the
// leader election object. Defaults to the same namespace in which the
// the controller runs.
func LeaderElectionNamespace(leaderElectionNamespace string) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.leaderElectionNamespace = leaderElectionNamespace
		return nil
	}
}

// LeaseDuration is the duration that non-leader candidates will
// wait to force acquire leadership. This is measured against time of
// last observed ack. Defaults to 15 seconds.
func LeaseDuration(leaseDuration time.Duration) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.leaseDuration = leaseDuration
		return nil
	}
}

// RenewDeadline is the duration that the acting master will retry
// refreshing leadership before giving up. Defaults to 10 seconds.
func RenewDeadline(renewDeadline time.Duration) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.renewDeadline = renewDeadline
		return nil
	}
}

// RetryPeriod is the duration the LeaderElector clients should wait
// between tries of actions. Defaults to 2 seconds.
func RetryPeriod(retryPeriod time.Duration) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.retryPeriod = retryPeriod
		return nil
	}
}

// ClaimsInformer sets the informer to use for accessing PersistentVolumeClaims.
// Defaults to using a internal informer.
func ClaimsInformer(informer cache.SharedInformer) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.claimInformer = informer
		c.customClaimInformer = true
		return nil
	}
}

// VolumesInformer sets the informer to use for accessing PersistentVolumes.
// Defaults to using a internal informer.
func VolumesInformer(informer cache.SharedInformer) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.volumeInformer = informer
		c.customVolumeInformer = true
		return nil
	}
}

// ClassesInformer sets the informer to use for accessing StorageClasses.
// The informer must use the versioned resource appropriate for the Kubernetes cluster version
// (that is, v1.StorageClass for >= 1.6, and v1beta1.StorageClass for < 1.6).
// Defaults to using a internal informer.
func ClassesInformer(informer cache.SharedInformer) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.classInformer = informer
		c.customClassInformer = true
		return nil
	}
}

// MetricsPort sets the port that metrics server serves on. Default: 0, set to non-zero to enable.
func MetricsPort(metricsPort int32) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.metricsPort = metricsPort
		return nil
	}
}

// MetricsAddress sets the ip address that metrics serve serves on.
func MetricsAddress(metricsAddress string) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.metricsAddress = metricsAddress
		return nil
	}
}

// MetricsPath sets the endpoint path of metrics server.
func MetricsPath(metricsPath string) func(*ProvisionController) error {
	return func(c *ProvisionController) error {
		if c.HasRun() {
			return errRuntime
		}
		c.metricsPath = metricsPath
		return nil
	}
}

// HasRun returns whether the controller has Run
func (ctrl *ProvisionController) HasRun() bool {
	ctrl.hasRunLock.Lock()
	defer ctrl.hasRunLock.Unlock()
	return ctrl.hasRun
}

// NewProvisionController creates a new provision controller using
// the given configuration parameters and with private (non-shared) informers.
func NewProvisionController(
	client kubernetes.Interface,
	cfg *option.Config,
	provisioners map[string]Provisioner,
	kubeVersion string,
	options ...func(*ProvisionController) error,
) *ProvisionController {
	id, err := os.Hostname()
	if err != nil {
		logrus.Fatalf("Error getting hostname: %v", err)
	}
	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id = id + "_" + string(uuid.NewUUID())
	component := "rainbond.io/provisioner" + "_" + id

	v1.AddToScheme(scheme.Scheme)
	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(logrus.Infof)
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: client.CoreV1().Events(v1.NamespaceAll)})
	eventRecorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: component})

	controller := &ProvisionController{
		client:                        client,
		cfg:                           cfg,
		provisioners:                  provisioners,
		kubeVersion:                   utilversion.MustParseSemantic(kubeVersion),
		id:                            id,
		component:                     component,
		eventRecorder:                 eventRecorder,
		resyncPeriod:                  DefaultResyncPeriod,
		exponentialBackOffOnError:     DefaultExponentialBackOffOnError,
		threadiness:                   DefaultThreadiness,
		createProvisionedPVRetryCount: DefaultCreateProvisionedPVRetryCount,
		createProvisionedPVInterval:   DefaultCreateProvisionedPVInterval,
		failedProvisionThreshold:      DefaultFailedProvisionThreshold,
		failedDeleteThreshold:         DefaultFailedDeleteThreshold,
		leaderElection:                DefaultLeaderElection,
		leaderElectionNamespace:       getInClusterNamespace(),
		leaseDuration:                 DefaultLeaseDuration,
		renewDeadline:                 DefaultRenewDeadline,
		retryPeriod:                   DefaultRetryPeriod,
		metricsPort:                   DefaultMetricsPort,
		metricsAddress:                DefaultMetricsAddress,
		metricsPath:                   DefaultMetricsPath,
		hasRun:                        false,
		hasRunLock:                    &sync.Mutex{},
	}

	for _, option := range options {
		option(controller)
	}

	ratelimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(15*time.Second, 1000*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)
	if !controller.exponentialBackOffOnError {
		ratelimiter = workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(15*time.Second, 15*time.Second),
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		)
	}
	controller.claimQueue = workqueue.NewNamedRateLimitingQueue(ratelimiter, "claims")
	controller.volumeQueue = workqueue.NewNamedRateLimitingQueue(ratelimiter, "volumes")

	informer := informers.NewSharedInformerFactory(client, controller.resyncPeriod)

	// ----------------------
	// PersistentVolumeClaims

	claimHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { controller.enqueueWork(controller.claimQueue, obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { controller.enqueueWork(controller.claimQueue, newObj) },
		DeleteFunc: func(obj interface{}) { controller.forgetWork(controller.claimQueue, obj) },
	}

	if controller.claimInformer != nil {
		controller.claimInformer.AddEventHandlerWithResyncPeriod(claimHandler, controller.resyncPeriod)
	} else {
		controller.claimInformer = informer.Core().V1().PersistentVolumeClaims().Informer()
		controller.claimInformer.AddEventHandler(claimHandler)
	}
	controller.claims = controller.claimInformer.GetStore()

	// -----------------
	// PersistentVolumes

	volumeHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { controller.enqueueWork(controller.volumeQueue, obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { controller.enqueueWork(controller.volumeQueue, newObj) },
		DeleteFunc: func(obj interface{}) {
			controller.forgetWork(controller.volumeQueue, obj)

			pv, ok := obj.(*v1.PersistentVolume)
			if !ok {
				logrus.Errorf("expected persistent volume but got %+v", obj)
				return
			}

			path, err := getDataPath(pv.Spec.PersistentVolumeSource)
			if err != nil {
				logrus.Errorf("get data path: %v", err)
				return
			}

			// rainbondsssc
			switch pv.Spec.StorageClassName {
			case "rainbondsssc":
				if err := os.RemoveAll(path); err != nil {
					logrus.Errorf("path: %s; name: %s; remove pv hostpath: %v", path, pv.Name, err)
				}
				logrus.Infof("storage class: rainbondsssc; path: %s; successfully delete pv hsot path", path)
			case "rainbondslsc":
				nodeIP := func() string {
					for _, me := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions {
						if me.Key != "kubernetes.io/hostname" {
							continue
						}
						return me.Values[0]
					}
					return ""
				}()

				if nodeIP == "" {
					logrus.Errorf("storage class: rainbondslsc; name: %s; node ip not found", pv.Name)
					return
				}

				if err := deletePath(nodeIP, path); err != nil {
					logrus.Errorf("delete path: %v", err)
					return
				}

				logrus.Infof("storage class: rainbondslsc; path: %s; successfully delete pv hsot path", path)
			default:
				logrus.Debugf("unsupported storage class: %s", pv.Spec.StorageClassName)
			}
		},
	}

	if controller.volumeInformer != nil {
		controller.volumeInformer.AddEventHandlerWithResyncPeriod(volumeHandler, controller.resyncPeriod)
	} else {
		controller.volumeInformer = informer.Core().V1().PersistentVolumes().Informer()
		controller.volumeInformer.AddEventHandler(volumeHandler)
	}
	controller.volumes = controller.volumeInformer.GetStore()

	// --------------
	// StorageClasses

	// no resource event handler needed for StorageClasses
	if controller.classInformer == nil {
		if controller.kubeVersion.AtLeast(utilversion.MustParseSemantic("v1.6.0")) {
			controller.classInformer = informer.Storage().V1().StorageClasses().Informer()
		} else {
			controller.classInformer = informer.Storage().V1beta1().StorageClasses().Informer()
		}
	}
	controller.classes = controller.classInformer.GetStore()

	return controller
}

// enqueueWork takes an obj and converts it into a namespace/name string which
// is then put onto the given work queue.
func (ctrl *ProvisionController) enqueueWork(queue workqueue.RateLimitingInterface, obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	// Re-Adding is harmless but try to add it to the queue only if it is not
	// already there, because if it is already there we *must* be retrying it
	if queue.NumRequeues(key) == 0 {
		queue.Add(key)
	}
}

// forgetWork Forgets an obj from the given work queue, telling the queue to
// stop tracking its retries because e.g. the obj was deleted
func (ctrl *ProvisionController) forgetWork(queue workqueue.RateLimitingInterface, obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	queue.Forget(key)
	queue.Done(key)
}

// Run starts all of this controller's control loops
func (ctrl *ProvisionController) Run(ctx context.Context) {
	// TODO: arg is as of 1.12 unused. Nothing can ever be cancelled. Should
	// accept a context instead and use it instead of context.TODO(), but would
	// break API. Not urgent: realistically, users are simply passing in
	// wait.NeverStop() anyway.
	stop := ctx.Done()
	logrus.Infof("Starting provisioner controller %s!", ctrl.component)
	defer utilruntime.HandleCrash()
	defer ctrl.claimQueue.ShutDown()
	defer ctrl.volumeQueue.ShutDown()

	ctrl.hasRunLock.Lock()
	ctrl.hasRun = true
	ctrl.hasRunLock.Unlock()

	// If a external SharedInformer has been passed in, this controller
	// should not call Run again
	if !ctrl.customClaimInformer {
		go ctrl.claimInformer.Run(stop)
	}
	if !ctrl.customVolumeInformer {
		go ctrl.volumeInformer.Run(stop)
	}
	if !ctrl.customClassInformer {
		go ctrl.classInformer.Run(stop)
	}

	if !cache.WaitForCacheSync(stop, ctrl.claimInformer.HasSynced, ctrl.volumeInformer.HasSynced, ctrl.classInformer.HasSynced) {
		return
	}

	for i := 0; i < ctrl.threadiness; i++ {
		go wait.Until(ctrl.runClaimWorker, time.Second, stop)
		go wait.Until(ctrl.runVolumeWorker, time.Second, stop)
	}
	logger := logrus.WithField("WHO", "ProvisionController")
	logger.Infof("Started provisioner controller %s!", ctrl.component)
	<-stop
	logger.Info("stop provisioner controller: %s", ctrl.component)
}

func (ctrl *ProvisionController) runClaimWorker() {
	for ctrl.processNextClaimWorkItem() {
	}
}

func (ctrl *ProvisionController) runVolumeWorker() {
	for ctrl.processNextVolumeWorkItem() {
	}
}

// processNextClaimWorkItem processes items from claimQueue
func (ctrl *ProvisionController) processNextClaimWorkItem() bool {
	obj, shutdown := ctrl.claimQueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer ctrl.claimQueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			ctrl.claimQueue.Forget(obj)
			return fmt.Errorf("expected string in workqueue but got %#v", obj)
		}

		if err := ctrl.syncClaimHandler(key); err != nil {
			if ctrl.claimQueue.NumRequeues(obj) < ctrl.failedProvisionThreshold {
				logrus.Warningf("Retrying syncing claim %q because failures %v < threshold %v",
					key, ctrl.claimQueue.NumRequeues(obj), ctrl.failedProvisionThreshold)
				ctrl.claimQueue.AddRateLimited(obj)
			} else {
				logrus.Errorf("Giving up syncing claim %q because failures %v >= threshold %v", key, ctrl.claimQueue.NumRequeues(obj), ctrl.failedProvisionThreshold)
				// Done but do not Forget: it will not be in the queue but NumRequeues
				// will be saved until the obj is deleted from kubernetes
			}
			return fmt.Errorf("error syncing claim %q: %s", key, err.Error())
		}

		ctrl.claimQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// processNextVolumeWorkItem processes items from volumeQueue
func (ctrl *ProvisionController) processNextVolumeWorkItem() bool {
	obj, shutdown := ctrl.volumeQueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer ctrl.volumeQueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			ctrl.volumeQueue.Forget(obj)
			return fmt.Errorf("expected string in workqueue but got %#v", obj)
		}

		if err := ctrl.syncVolumeHandler(key); err != nil {
			if ctrl.volumeQueue.NumRequeues(obj) < ctrl.failedDeleteThreshold {
				logrus.Warningf("Retrying syncing volume %q because failures %v < threshold %v", key, ctrl.volumeQueue.NumRequeues(obj), ctrl.failedProvisionThreshold)
				ctrl.volumeQueue.AddRateLimited(obj)
			} else {
				logrus.Errorf("Giving up syncing volume %q because failures %v >= threshold %v", key, ctrl.volumeQueue.NumRequeues(obj), ctrl.failedProvisionThreshold)
				// Done but do not Forget: it will not be in the queue but NumRequeues
				// will be saved until the obj is deleted from kubernetes
			}
			return fmt.Errorf("error syncing volume %q: %s", key, err.Error())
		}

		ctrl.volumeQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncClaimHandler gets the claim from informer's cache then calls syncClaim
func (ctrl *ProvisionController) syncClaimHandler(key string) error {
	claimObj, exists, err := ctrl.claims.GetByKey(key)
	if err != nil {
		return err
	}
	if !exists {
		utilruntime.HandleError(fmt.Errorf("claim %q in work queue no longer exists", key))
		return nil
	}

	return ctrl.syncClaim(claimObj)
}

// syncVolumeHandler gets the volume from informer's cache then calls syncVolume
func (ctrl *ProvisionController) syncVolumeHandler(key string) error {
	volumeObj, exists, err := ctrl.volumes.GetByKey(key)
	if err != nil {
		return err
	}
	if !exists {
		utilruntime.HandleError(fmt.Errorf("volume %q in work queue no longer exists", key))
		return nil
	}

	return ctrl.syncVolume(volumeObj)
}

// syncClaim checks if the claim should have a volume provisioned for it and
// provisions one if so.
func (ctrl *ProvisionController) syncClaim(obj interface{}) error {
	claim, ok := obj.(*v1.PersistentVolumeClaim)
	if !ok {
		return fmt.Errorf("expected claim but got %+v", obj)
	}

	if ctrl.shouldProvision(claim) {
		startTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*100)
		defer cancel()
		err := ctrl.provisionClaimOperation(ctx, claim)
		ctrl.updateProvisionStats(claim, err, startTime)
		return err
	}
	return nil
}

// syncVolume checks if the volume should be deleted and deletes if so
func (ctrl *ProvisionController) syncVolume(obj interface{}) error {
	volume, ok := obj.(*v1.PersistentVolume)
	if !ok {
		return fmt.Errorf("expected volume but got %+v", obj)
	}
	if ctrl.shouldDelete(volume) {
		startTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()
		err := ctrl.deleteVolumeOperation(ctx, volume)
		ctrl.updateDeleteStats(volume, err, startTime)
		return err
	}
	return nil
}

// shouldProvision returns whether a claim should have a volume provisioned for
// it, i.e. whether a Provision is "desired"
func (ctrl *ProvisionController) shouldProvision(claim *v1.PersistentVolumeClaim) bool {
	if claim.Spec.VolumeName != "" {
		return false
	}
	// Kubernetes 1.5 provisioning with annStorageProvisioner
	if ctrl.kubeVersion.AtLeast(utilversion.MustParseSemantic("v1.5.0")) {
		if provisioner, found := claim.Annotations[annStorageProvisioner]; found {
			if pr, ok := ctrl.provisioners[provisioner]; ok {
				if qualifier, ok := pr.(Qualifier); ok {
					if !qualifier.ShouldProvision(claim) {
						return false
					}
				}
				return true
			}
			return false
		}
	} else {
		return true
	}

	return false
}

// shouldDelete returns whether a volume should have its backing volume
// deleted, i.e. whether a Delete is "desired"
func (ctrl *ProvisionController) shouldDelete(volume *v1.PersistentVolume) bool {
	// In 1.9+ PV protection means the object will exist briefly with a
	// deletion timestamp even after our successful Delete. Ignore it.
	if ctrl.kubeVersion.AtLeast(utilversion.MustParseSemantic("v1.9.0")) {
		if volume.ObjectMeta.DeletionTimestamp != nil {
			return false
		}
	}

	// In 1.5+ we delete only if the volume is in state Released. In 1.4 we must
	// delete if the volume is in state Failed too.
	if ctrl.kubeVersion.AtLeast(utilversion.MustParseSemantic("v1.5.0")) {
		if volume.Status.Phase != v1.VolumeReleased {
			return false
		}
	} else {
		if volume.Status.Phase != v1.VolumeReleased && volume.Status.Phase != v1.VolumeFailed {
			return false
		}
	}

	if volume.Spec.PersistentVolumeReclaimPolicy != v1.PersistentVolumeReclaimDelete {
		return false
	}

	if !metav1.HasAnnotation(volume.ObjectMeta, annDynamicallyProvisioned) {
		return false
	}

	if ctrl.getProvisioner(volume) == nil {
		return false
	}

	return true
}
func (ctrl *ProvisionController) getProvisioner(volume *v1.PersistentVolume) Provisioner {
	if ann := volume.Annotations[annDynamicallyProvisioned]; ann != "" {
		if pr, ok := ctrl.provisioners[ann]; ok {
			return pr
		}
	}
	return nil
}
func (ctrl *ProvisionController) getProvisionerByPvc(pvc *v1.PersistentVolumeClaim) Provisioner {
	if ann := pvc.Annotations[annDynamicallyProvisioned]; ann != "" {
		if pr, ok := ctrl.provisioners[ann]; ok {
			return pr
		}
	}
	return nil
}

// canProvision returns error if provisioner can't provision claim.
func (ctrl *ProvisionController) canProvision(claim *v1.PersistentVolumeClaim, pr Provisioner) error {
	// Check if this provisioner supports Block volume
	if util.CheckPersistentVolumeClaimModeBlock(claim) && !ctrl.supportsBlock(pr) {
		return fmt.Errorf("%s does not support block volume provisioning", pr.Name())
	}

	return nil
}

func (ctrl *ProvisionController) updateProvisionStats(claim *v1.PersistentVolumeClaim, err error, startTime time.Time) {
	class := ""
	if claim.Spec.StorageClassName != nil {
		class = *claim.Spec.StorageClassName
	}
	if err != nil {
		metrics.PersistentVolumeClaimProvisionFailedTotal.WithLabelValues(class).Inc()
	} else {
		metrics.PersistentVolumeClaimProvisionDurationSeconds.WithLabelValues(class).Observe(time.Since(startTime).Seconds())
		metrics.PersistentVolumeClaimProvisionTotal.WithLabelValues(class).Inc()
	}
}

func (ctrl *ProvisionController) updateDeleteStats(volume *v1.PersistentVolume, err error, startTime time.Time) {
	class := volume.Spec.StorageClassName
	if err != nil {
		metrics.PersistentVolumeDeleteFailedTotal.WithLabelValues(class).Inc()
	} else {
		metrics.PersistentVolumeDeleteDurationSeconds.WithLabelValues(class).Observe(time.Since(startTime).Seconds())
		metrics.PersistentVolumeDeleteTotal.WithLabelValues(class).Inc()
	}
}

// GetPersistentVolumeClaimClass returns StorageClassName. If no storage class was
// requested, it returns "".
func GetPersistentVolumeClaimClass(claim *v1.PersistentVolumeClaim) string {
	// Use beta annotation first
	if class, found := claim.Annotations[v1.BetaStorageClassAnnotation]; found {
		return class
	}

	if claim.Spec.StorageClassName != nil {
		return *claim.Spec.StorageClassName
	}

	return ""
}

// provisionClaimOperation attempts to provision a volume for the given claim.
// Returns error, which indicates whether provisioning should be retried
// (requeue the claim) or not
func (ctrl *ProvisionController) provisionClaimOperation(ctx context.Context, claim *v1.PersistentVolumeClaim) error {
	// Most code here is identical to that found in controller.go of kube's PV controller...
	claimClass := GetPersistentVolumeClaimClass(claim)
	operation := fmt.Sprintf("provision %q class %q", claimToClaimKey(claim), claimClass)
	logrus.Info(logOperation(operation, "started"))

	//  A previous doProvisionClaim may just have finished while we were waiting for
	//  the locks. Check that PV (with deterministic name) hasn't been provisioned
	//  yet.
	pvName := ctrl.getProvisionedVolumeNameForClaim(claim)
	volume, err := ctrl.client.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err == nil && volume != nil {
		// Volume has been already provisioned, nothing to do.
		logrus.Info(logOperation(operation, "persistentvolume %q already exists, skipping", pvName))
		return nil
	}

	// Prepare a claimRef to the claim early (to fail before a volume is
	// provisioned)
	claimRef, err := ref.GetReference(scheme.Scheme, claim)
	if err != nil {
		logrus.Error(logOperation(operation, "unexpected error getting claim reference: %v", err))
		return nil
	}

	provisioner, parameters, err := ctrl.getStorageClassFields(claimClass)
	if err != nil {
		logrus.Error(logOperation(operation, "error getting claim's StorageClass's fields: %v", err))
		return nil
	}
	if _, ok := ctrl.provisioners[provisioner]; !ok {
		// class.Provisioner has either changed since shouldProvision() or
		// annDynamicallyProvisioned contains different provisioner than
		// class.Provisioner.
		logrus.Error(logOperation(operation, "unknown provisioner %q requested in claim's StorageClass", provisioner))
		return nil
	}

	// Check if this provisioner can provision this claim.
	if err = ctrl.canProvision(claim, ctrl.provisioners[provisioner]); err != nil {
		ctrl.eventRecorder.Event(claim, v1.EventTypeWarning, "ProvisioningFailed", err.Error())
		logrus.Error(logOperation(operation, "failed to provision volume: %v", err))
		return nil
	}

	reclaimPolicy := v1.PersistentVolumeReclaimDelete
	if ctrl.kubeVersion.AtLeast(utilversion.MustParseSemantic("v1.8.0")) {
		reclaimPolicy, err = ctrl.fetchReclaimPolicy(claimClass)
		if err != nil {
			return err
		}
	}

	mountOptions, err := ctrl.fetchMountOptions(claimClass)
	if err != nil {
		return err
	}

	var selectedNode *v1.Node
	var allowedTopologies []v1.TopologySelectorTerm
	if ctrl.kubeVersion.AtLeast(utilversion.MustParseSemantic("v1.11.0")) {
		// Get SelectedNode
		if nodeName, ok := claim.Annotations[annSelectedNode]; ok {
			selectedNode, err = ctrl.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{}) // TODO (verult) cache Nodes
			if err != nil {
				err = fmt.Errorf("failed to get target node: %v", err)
				ctrl.eventRecorder.Event(claim, v1.EventTypeWarning, "ProvisioningFailed", err.Error())
				return err
			}
		}

		// Get AllowedTopologies
		allowedTopologies, err = ctrl.fetchAllowedTopologies(claimClass)
		if err != nil {
			err = fmt.Errorf("failed to get AllowedTopologies from StorageClass: %v", err)
			ctrl.eventRecorder.Event(claim, v1.EventTypeWarning, "ProvisioningFailed", err.Error())
			return err
		}
	}

	// Find pv for grdata
	grdatapv, err := ctrl.persistentVolumeForGrdata(ctx)
	if err != nil {
		return fmt.Errorf("pv for grdata: %v", err)
	}

	options := VolumeOptions{
		PersistentVolumeReclaimPolicy: reclaimPolicy,
		PVName:                        pvName,
		PVC:                           claim,
		PersistentVolumeSource:        grdatapv.Spec.PersistentVolumeSource,
		MountOptions:                  mountOptions,
		Parameters:                    parameters,
		SelectedNode:                  selectedNode,
		AllowedTopologies:             allowedTopologies,
	}

	ctrl.eventRecorder.Event(claim, v1.EventTypeNormal, "Provisioning", fmt.Sprintf("External provisioner is provisioning volume for claim %q", claimToClaimKey(claim)))

	volume, err = ctrl.provisioners[provisioner].Provision(options)
	if err != nil {
		if err == dao.ErrVolumeNotFound {
			logrus.Warningf("PVC: %s; volume not found.", claim.Name)
			return nil
		}
		if ierr, ok := err.(*IgnoredError); ok {
			// Provision ignored, do nothing and hope another provisioner will provision it.
			logrus.Info(logOperation(operation, "volume provision ignored: %v", ierr))
			return nil
		}
		err = fmt.Errorf("failed to provision volume with StorageClass %q: %v", claimClass, err)
		ctrl.eventRecorder.Event(claim, v1.EventTypeWarning, "ProvisioningFailed", err.Error())
		return err
	}

	logrus.Info(logOperation(operation, "volume %q provisioned", volume.Name))

	// Set ClaimRef and the PV controller will bind and set annBoundByController for us
	volume.Spec.ClaimRef = claimRef

	metav1.SetMetaDataAnnotation(&volume.ObjectMeta, annDynamicallyProvisioned, ctrl.provisioners[provisioner].Name())
	if ctrl.kubeVersion.AtLeast(utilversion.MustParseSemantic("v1.6.0")) {
		volume.Spec.StorageClassName = claimClass
	} else {
		metav1.SetMetaDataAnnotation(&volume.ObjectMeta, annClass, claimClass)
	}

	// Try to create the PV object several times
	for i := 0; i < ctrl.createProvisionedPVRetryCount; i++ {
		logrus.Info(logOperation(operation, "trying to save persistentvolume %q", volume.Name))
		if _, err = ctrl.client.CoreV1().PersistentVolumes().Create(ctx, volume, metav1.CreateOptions{}); err == nil || apierrs.IsAlreadyExists(err) {
			// Save succeeded.
			if err != nil {
				logrus.Info(logOperation(operation, "persistentvolume %q already exists, reusing", volume.Name))
				err = nil
			} else {
				logrus.Info(logOperation(operation, "persistentvolume %q saved", volume.Name))
			}
			break
		}
		// Save failed, try again after a while.
		logrus.Info(logOperation(operation, "failed to save persistentvolume %q: %v", volume.Name, err))
		time.Sleep(ctrl.createProvisionedPVInterval)
	}

	if err != nil {
		// Save failed. Now we have a storage asset outside of Kubernetes,
		// but we don't have appropriate PV object for it.
		// Emit some event here and try to delete the storage asset several
		// times.
		strerr := fmt.Sprintf("Error creating provisioned PV object for claim %s: %v. Deleting the volume.", claimToClaimKey(claim), err)
		logrus.Error(logOperation(operation, strerr))
		ctrl.eventRecorder.Event(claim, v1.EventTypeWarning, "ProvisioningFailed", strerr)

		for i := 0; i < ctrl.createProvisionedPVRetryCount; i++ {
			if err = ctrl.provisioners[provisioner].Delete(volume); err == nil {
				// Delete succeeded
				logrus.Info(logOperation(operation, "cleaning volume %q succeeded", volume.Name))
				break
			}
			// Delete failed, try again after a while.
			logrus.Info(logOperation(operation, "failed to clean volume %q: %v", volume.Name, err))
			time.Sleep(ctrl.createProvisionedPVInterval)
		}

		if err != nil {
			// Delete failed several times. There is an orphaned volume and there
			// is nothing we can do about it.
			strerr := fmt.Sprintf("Error cleaning provisioned volume for claim %s: %v. Please delete manually.", claimToClaimKey(claim), err)
			logrus.Error(logOperation(operation, strerr))
			ctrl.eventRecorder.Event(claim, v1.EventTypeWarning, "ProvisioningCleanupFailed", strerr)
		}
	} else {
		msg := fmt.Sprintf("Successfully provisioned volume %s", volume.Name)
		ctrl.eventRecorder.Event(claim, v1.EventTypeNormal, "ProvisioningSucceeded", msg)
	}

	logrus.Info(logOperation(operation, "succeeded"))
	return nil
}

func (ctrl *ProvisionController) persistentVolumeForGrdata(ctx context.Context) (*v1.PersistentVolume, error) {
	pvc, err := ctrl.client.CoreV1().PersistentVolumeClaims(ctrl.cfg.RBDNamespace).Get(ctx, ctrl.cfg.GrdataPVCName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("find pvc for grdata: %v", err)
	}
	pv, err := ctrl.client.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("find pv for grdata: %v", err)
	}
	if pv.Spec.PersistentVolumeSource.Glusterfs != nil && pv.Spec.PersistentVolumeSource.Glusterfs.EndpointsNamespace == nil {
		pv.Spec.PersistentVolumeSource.Glusterfs.EndpointsNamespace = &pv.Spec.ClaimRef.Namespace
	}
	return pv, nil
}

// deleteVolumeOperation attempts to delete the volume backing the given
// volume. Returns error, which indicates whether deletion should be retried
// (requeue the volume) or not
func (ctrl *ProvisionController) deleteVolumeOperation(ctx context.Context, volume *v1.PersistentVolume) error {
	operation := fmt.Sprintf("delete %q", volume.Name)
	logrus.Info(logOperation(operation, "started"))

	// This method may have been waiting for a volume lock for some time.
	// Our check does not have to be as sophisticated as PV controller's, we can
	// trust that the PV controller has set the PV to Released/Failed and it's
	// ours to delete
	newVolume, err := ctrl.client.CoreV1().PersistentVolumes().Get(ctx, volume.Name, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	if !ctrl.shouldDelete(newVolume) {
		logrus.Info(logOperation(operation, "persistentvolume no longer needs deletion, skipping"))
		return nil
	}

	err = ctrl.getProvisioner(volume).Delete(volume)
	if err != nil {
		if ierr, ok := err.(*IgnoredError); ok {
			// Delete ignored, do nothing and hope another provisioner will delete it.
			logrus.Info(logOperation(operation, "volume deletion ignored: %v", ierr))
			return nil
		}
		// Delete failed, emit an event.
		logrus.Error(logOperation(operation, "volume deletion failed: %v", err))
		ctrl.eventRecorder.Event(volume, v1.EventTypeWarning, "VolumeFailedDelete", err.Error())
		return err
	}

	logrus.Info(logOperation(operation, "volume deleted"))

	// Delete the volume
	if err = ctrl.client.CoreV1().PersistentVolumes().Delete(ctx, volume.Name, metav1.DeleteOptions{}); err != nil {
		// Oops, could not delete the volume and therefore the controller will
		// try to delete the volume again on next update.
		logrus.Info(logOperation(operation, "failed to delete persistentvolume: %v", err))
		return err
	}

	logrus.Info(logOperation(operation, "persistentvolume deleted"))

	logrus.Info(logOperation(operation, "succeeded"))
	return nil
}

func logOperation(operation, format string, a ...interface{}) string {
	return fmt.Sprintf(fmt.Sprintf("%s: %s", operation, format), a...)
}

// getInClusterNamespace returns the namespace in which the controller runs.
func getInClusterNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return "default"
}

// getProvisionedVolumeNameForClaim returns PV.Name for the provisioned volume.
// The name must be unique.
func (ctrl *ProvisionController) getProvisionedVolumeNameForClaim(claim *v1.PersistentVolumeClaim) string {
	return "pvc-" + string(claim.UID)
}

func (ctrl *ProvisionController) getStorageClassFields(name string) (string, map[string]string, error) {
	classObj, found, err := ctrl.classes.GetByKey(name)
	if err != nil {
		return "", nil, err
	}
	if !found {
		return "", nil, fmt.Errorf("storageClass %q not found", name)
		// 3. It tries to find a StorageClass instance referenced by annotation
		//    `claim.Annotations["volume.beta.kubernetes.io/storage-class"]`. If not
		//    found, it SHOULD report an error (by sending an event to the claim) and it
		//    SHOULD retry periodically with step i.
	}
	switch class := classObj.(type) {
	case *storage.StorageClass:
		return class.Provisioner, class.Parameters, nil
	case *storagebeta.StorageClass:
		return class.Provisioner, class.Parameters, nil
	}
	return "", nil, fmt.Errorf("cannot convert object to StorageClass: %+v", classObj)
}

func claimToClaimKey(claim *v1.PersistentVolumeClaim) string {
	return fmt.Sprintf("%s/%s", claim.Namespace, claim.Name)
}

func (ctrl *ProvisionController) fetchReclaimPolicy(storageClassName string) (v1.PersistentVolumeReclaimPolicy, error) {
	classObj, found, err := ctrl.classes.GetByKey(storageClassName)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("storageClass %q not found", storageClassName)
	}

	switch class := classObj.(type) {
	case *storage.StorageClass:
		return *class.ReclaimPolicy, nil
	case *storagebeta.StorageClass:
		return *class.ReclaimPolicy, nil
	}

	return v1.PersistentVolumeReclaimDelete, fmt.Errorf("cannot convert object to StorageClass: %+v", classObj)
}

func (ctrl *ProvisionController) fetchMountOptions(storageClassName string) ([]string, error) {
	classObj, found, err := ctrl.classes.GetByKey(storageClassName)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("storageClass %q not found", storageClassName)
	}

	switch class := classObj.(type) {
	case *storage.StorageClass:
		return class.MountOptions, nil
	case *storagebeta.StorageClass:
		return class.MountOptions, nil
	}

	return nil, fmt.Errorf("cannot convert object to StorageClass: %+v", classObj)
}

func (ctrl *ProvisionController) fetchAllowedTopologies(storageClassName string) ([]v1.TopologySelectorTerm, error) {
	classObj, found, err := ctrl.classes.GetByKey(storageClassName)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("storageClass %q not found", storageClassName)
	}

	switch class := classObj.(type) {
	case *storage.StorageClass:
		return class.AllowedTopologies, nil
	case *storagebeta.StorageClass:
		return class.AllowedTopologies, nil
	}

	return nil, fmt.Errorf("cannot convert object to StorageClass: %+v", classObj)
}

// supportsBlock returns whether a provisioner supports block volume.
// Provisioners that implement BlockProvisioner interface and return true to SupportsBlock
// will be regarded as supported for block volume.
func (ctrl *ProvisionController) supportsBlock(pr Provisioner) bool {
	if blockProvisioner, ok := pr.(BlockProvisioner); ok {
		return blockProvisioner.SupportsBlock()
	}
	return false
}

func deletePath(nodeIP, path string) error {
	logrus.Infof("node ip: %s; path: %s; delete pv hostpath", nodeIP, path)

	reqOpts := map[string]string{
		"path": path,
	}

	retry := 3
	var err error
	var statusCode int
	for retry > 0 {
		retry--
		body := bytes.NewBuffer(nil)
		if err := json.NewEncoder(body).Encode(reqOpts); err != nil {
			return err
		}

		// create request
		var req *http.Request
		req, err = http.NewRequest("DELETE", fmt.Sprintf("http://%s:6100/v2/localvolumes", nodeIP), body)
		if err != nil {
			logrus.Warningf("remaining retry times: %d; path: %s; new request: %v", retry, path, err)
			continue
		}

		var res *http.Response
		res, err = http.DefaultClient.Do(req)
		if err != nil {
			logrus.Warningf("remaining retry times: %d; path: %s; do http request: %v", retry, path, err)
			continue
		}
		defer res.Body.Close()

		statusCode = res.StatusCode
		if statusCode == 200 {
			return nil
		}

		logrus.Warningf("remaining retry times: %d; path: %s; status code: %d; delete local volume", retry, path, res.StatusCode)
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("delete local path: %s; status code: %d; err: %v", path, statusCode, err)
}

func getDataPath(source v1.PersistentVolumeSource) (string, error) {
	switch {
	case source.HostPath != nil:
		return source.HostPath.Path, nil
	case source.NFS != nil:
		return source.NFS.Path, nil
	case source.Glusterfs != nil:
		//glusterfs:
		//	endpoints: glusterfs-cluster
		//	path: myVol1
		return source.Glusterfs.Path, nil
	default:
		return "", fmt.Errorf("unsupported persistence volume source")
	}
}
