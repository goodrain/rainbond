package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *ServiceAction) getServicePods(serviceID string) (*pb.ServiceAppPodList, error) {
	if s != nil && s.getServicePodsHook != nil {
		return s.getServicePodsHook(serviceID)
	}
	return s.statusCli.GetServicePods(serviceID)
}

func (s *ServiceAction) cleanupCompletedVMLauncherPods(serviceID string) map[string]struct{} {
	excluded := make(map[string]struct{})
	if s == nil || s.kubeClient == nil || strings.TrimSpace(serviceID) == "" {
		return excluded
	}

	pods, err := s.kubeClient.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("service_id=%s,kubevirt.io=virt-launcher", serviceID),
	})
	if err != nil {
		logrus.Warningf("list vm launcher pods for cleanup failure: service_id=%s err=%v", serviceID, err)
		return excluded
	}

	hasRunningLauncher := false
	for i := range pods.Items {
		if pods.Items[i].Status.Phase == corev1.PodRunning {
			hasRunningLauncher = true
			break
		}
	}
	if !hasRunningLauncher {
		return excluded
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase != corev1.PodSucceeded {
			continue
		}
		excluded[pod.Name] = struct{}{}
		if pod.DeletionTimestamp != nil {
			continue
		}
		if err := s.kubeClient.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{}); err != nil && !k8sErrors.IsNotFound(err) {
			logrus.Warningf("delete completed vm launcher pod failure: %s/%s err=%v", pod.Namespace, pod.Name, err)
		}
	}
	return excluded
}

func (s *ServiceAction) enqueueCompletedVMLauncherCleanup(serviceID string) {
	if s == nil || s.kubeClient == nil || strings.TrimSpace(serviceID) == "" {
		return
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		timeout := time.After(2 * time.Minute)
		for {
			if len(s.cleanupCompletedVMLauncherPods(serviceID)) > 0 {
				return
			}
			select {
			case <-ticker.C:
			case <-timeout:
				return
			}
		}
	}()
}

func filterK8sPodInfos(pods []*K8sPodInfo, excluded map[string]struct{}) []*K8sPodInfo {
	if len(pods) == 0 || len(excluded) == 0 {
		return pods
	}
	filtered := make([]*K8sPodInfo, 0, len(pods))
	for _, pod := range pods {
		if pod == nil {
			continue
		}
		if _, ok := excluded[pod.PodName]; ok {
			continue
		}
		filtered = append(filtered, pod)
	}
	return filtered
}
