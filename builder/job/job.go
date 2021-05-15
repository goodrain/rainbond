// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package job

import (
	"bufio"
	"context"
	"io"
	"sync"
	"time"

	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

//Controller build job controller
type Controller interface {
	ExecJob(ctx context.Context, job *corev1.Pod, logger io.Writer, result *channels.RingChannel) error
	GetJob(string) (*corev1.Pod, error)
	GetServiceJobs(serviceID string) ([]*corev1.Pod, error)
	DeleteJob(job string)
	GetLanguageBuildSetting(ctx context.Context, lang code.Lang, name string) string
	GetDefaultLanguageBuildSetting(ctx context.Context, lang code.Lang) string
}
type controller struct {
	KubeClient         kubernetes.Interface
	ctx                context.Context
	jobInformer        v1.PodInformer
	namespace          string
	subJobStatus       sync.Map
	jobContainerStatus sync.Map
}

var jobController *controller

//InitJobController init job controller
func InitJobController(rbdNamespace string, stop chan struct{}, kubeClient kubernetes.Interface) error {
	jobController = &controller{
		KubeClient: kubeClient,
		namespace:  rbdNamespace,
	}
	logrus.Infof("watch namespace[%s] job ", rbdNamespace)
	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			job, _ := obj.(*corev1.Pod)
			logrus.Infof("[Watch] Build job pod %s created", job.Name)
		},
		DeleteFunc: func(obj interface{}) {
			job, _ := obj.(*corev1.Pod)
			if val, exist := jobController.subJobStatus.Load(job.Name); exist {
				ch := val.(*channels.RingChannel)
				ch.In() <- "cancel"
			}
			logrus.Infof("[Watch] Build job pod %s deleted", job.Name)
		},
		UpdateFunc: func(old, cur interface{}) {
			job, _ := cur.(*corev1.Pod)
			if len(job.Status.ContainerStatuses) > 0 {
				buildContainer := job.Status.ContainerStatuses[0]
				logrus.Infof("job %s container %s state %+v", job.Name, buildContainer.Name, buildContainer.State)
				terminated := buildContainer.State.Terminated
				if terminated != nil && terminated.ExitCode == 0 {
					if val, exist := jobController.subJobStatus.Load(job.Name); exist {
						logrus.Infof("job %s container exit 0 and complete", job.Name)
						ch := val.(*channels.RingChannel)
						ch.In() <- "complete"
					}
				}
				if terminated != nil && terminated.ExitCode > 0 {
					if val, exist := jobController.subJobStatus.Load(job.Name); exist {
						logrus.Infof("job[%s] container exit %d and failed", job.Name, terminated.ExitCode)
						ch := val.(*channels.RingChannel)
						ch.In() <- "failed"
					}
				}
				waiting := buildContainer.State.Waiting
				if waiting != nil && waiting.Reason == "CrashLoopBackOff" {
					logrus.Infof("job %s container status is waiting and reason is CrashLoopBackOff", job.Name)
					if val, exist := jobController.subJobStatus.Load(job.Name); exist {
						ch := val.(*channels.RingChannel)
						ch.In() <- "failed"
					}
				}
				if buildContainer.State.Running != nil || terminated != nil || (waiting != nil && waiting.Reason == "CrashLoopBackOff") {
					// job container is ready
					if val, exist := jobController.jobContainerStatus.Load(job.Name); exist {
						jobContainerCh := val.(chan struct{})
						// no block channel write
						select {
						case jobContainerCh <- struct{}{}:
						default:
							// if channel is block, ignore it
						}
					}
				}
			}
		},
	}
	infFactory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		time.Second*10,
		informers.WithNamespace(jobController.namespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = "job=codebuild"
		}))
	jobController.jobInformer = infFactory.Core().V1().Pods()
	jobController.jobInformer.Informer().AddEventHandlerWithResyncPeriod(eventHandler, time.Second*10)
	return jobController.Start(stop)
}

//GetJobController get job controller
func GetJobController() Controller {
	return jobController
}

func (c *controller) GetJob(name string) (*corev1.Pod, error) {
	return c.jobInformer.Lister().Pods(c.namespace).Get(name)
}

func (c *controller) GetServiceJobs(serviceID string) ([]*corev1.Pod, error) {
	s, err := labels.Parse("service=" + serviceID)
	if err != nil {
		return nil, err
	}
	jobs, err := c.jobInformer.Lister().Pods(c.namespace).List(s)
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

func (c *controller) ExecJob(ctx context.Context, job *corev1.Pod, logger io.Writer, result *channels.RingChannel) error {
	// one job, one job container channel
	jobContainerCh := make(chan struct{}, 1)
	c.jobContainerStatus.Store(job.Name, jobContainerCh)
	if j, _ := c.GetJob(job.Name); j != nil {
		go c.getLogger(ctx, job.Name, logger, result, jobContainerCh)
		c.subJobStatus.Store(job.Name, result)
		return nil
	}
	_, err := c.KubeClient.CoreV1().Pods(c.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	go c.getLogger(ctx, job.Name, logger, result, jobContainerCh)
	c.subJobStatus.Store(job.Name, result)
	return nil
}

func (c *controller) Start(stop chan struct{}) error {
	go c.jobInformer.Informer().Run(stop)
	for !c.jobInformer.Informer().HasSynced() {
		time.Sleep(time.Millisecond * 500)
	}
	return nil
}

func (c *controller) getLogger(ctx context.Context, job string, writer io.Writer, result *channels.RingChannel, jobContainerCh chan struct{}) {
	defer func() {
		logrus.Infof("job[%s] get log complete", job)
		result.In() <- "logcomplete"
	}()
	for {
		select {
		case <-ctx.Done():
			logrus.Debugf("job[%s] task is done, exit get log func", job)
			return
		case <-jobContainerCh:
			// reader log just only do once, if complete, exit this func
			logrus.Debugf("job[%s] container is ready, start get log stream", job)
			podLogRequest := c.KubeClient.CoreV1().Pods(c.namespace).GetLogs(job, &corev1.PodLogOptions{Follow: true})
			reader, err := podLogRequest.Stream(ctx)
			if err != nil {
				logrus.Warnf("get build job pod log data error: %s", err.Error())
				return
			}
			logrus.Debugf("get job[%s] log stream successfully, ready for reading log", job)
			defer reader.Close()
			bufReader := bufio.NewReader(reader)
			for {
				line, err := bufReader.ReadBytes('\n')
				if err == io.EOF {
					logrus.Debugf("job[%s] get log eof", job)
					return
				}
				if err != nil {
					logrus.Warningf("get job log error: %s", err.Error())
					return
				}
				writer.Write(line)
			}
		}
	}
}

func (c *controller) DeleteJob(job string) {
	namespace := c.namespace
	logrus.Debugf("start delete job: %s", job)
	// delete job
	if err := c.KubeClient.CoreV1().Pods(namespace).Delete(context.Background(), job, metav1.DeleteOptions{}); err != nil {
		if !k8sErrors.IsNotFound(err) {
			logrus.Errorf("delete job failed: %s", err.Error())
		}
	}
	c.subJobStatus.Delete(job)
	c.jobContainerStatus.Delete(job)
	logrus.Infof("delete job %s finish", job)
}

func (c *controller) GetLanguageBuildSetting(ctx context.Context, lang code.Lang, name string) string {
	config, err := c.KubeClient.CoreV1().ConfigMaps(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("get configmap %s failure  %s", name, err.Error())
		return ""
	}
	if config != nil {
		return name
	}
	return ""
}

func (c *controller) GetDefaultLanguageBuildSetting(ctx context.Context, lang code.Lang) string {
	config, err := c.KubeClient.CoreV1().ConfigMaps(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "default=true",
	})
	if err != nil {
		logrus.Errorf("get  default maven setting configmap failure  %s", err.Error())
	}
	if config != nil {
		for _, c := range config.Items {
			return c.Name
		}
	}
	return ""
}
