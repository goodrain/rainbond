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

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm/store"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//ErrWaitTimeOut wait time out
var ErrWaitTimeOut = fmt.Errorf("Wait time out")

//ErrWaitCancel wait cancel
var ErrWaitCancel = fmt.Errorf("Wait cancel")

//WaitReady wait ready
func WaitReady(store store.Storer, a *v1.AppService, timeout time.Duration, logger event.Logger, cancel chan struct{}) error {
	return WaitReadyWithClient(store, a, timeout, logger, cancel, nil)
}

//WaitReadyWithClient wait ready with kubernetes client for checking events
func WaitReadyWithClient(store store.Storer, a *v1.AppService, timeout time.Duration, logger event.Logger, cancel chan struct{}, client kubernetes.Interface) error {
	if timeout < 40 {
		timeout = time.Second * 40
	}
	logger.Info(fmt.Sprintf("waiting app ready timeout %ds", int(timeout.Seconds())), map[string]string{"step": "appruntime", "status": "running"})
	logrus.Debugf("waiting app ready timeout %ds", int(timeout.Seconds()))
	ticker := time.NewTicker(timeout / 10)
	timer := time.NewTimer(timeout)
	defer ticker.Stop()
	var i int
	var noPodCreatedCount int
	for {
		if i > 2 {
			a = store.UpdateGetAppService(a.ServiceID)
		}
		if a.Ready() {
			return nil
		}

		// Critical: Check if no Pods are created at all
		pods := a.GetPods(false)
		if len(pods) == 0 && i >= 3 && client != nil {
			noPodCreatedCount++
			// If no pods created after 3 checks (30% of timeout), investigate immediately
			if noPodCreatedCount >= 3 {
				if err := checkWorkloadFailureReason(client, a, logger); err != nil {
					return err
				}
			}
		} else {
			noPodCreatedCount = 0
		}

		printLogger(a, logger)
		select {
		case <-cancel:
			return ErrWaitCancel
		case <-timer.C:
			// Timeout: investigate why it failed before returning generic error
			if client != nil {
				if err := checkWorkloadFailureReason(client, a, logger); err != nil {
					return err
				}
			}
			return ErrWaitTimeOut
		case <-ticker.C:
		}
		i++
	}
}

//WaitStop wait service stop complete
func WaitStop(store store.Storer, a *v1.AppService, timeout time.Duration, logger event.Logger, cancel chan struct{}) error {
	if a == nil {
		return nil
	}
	if timeout < 40 {
		timeout = time.Second * 40
	}
	logger.Info(fmt.Sprintf("waiting app closed timeout %ds", int(timeout.Seconds())), map[string]string{"step": "appruntime", "status": "running"})
	logrus.Debugf("waiting app ready timeout %ds", int(timeout.Seconds()))
	ticker := time.NewTicker(timeout / 10)
	timer := time.NewTimer(timeout)
	defer ticker.Stop()
	var i int
	for {
		i++
		if i > 2 {
			a = store.UpdateGetAppService(a.ServiceID)
		}
		if a.IsClosed() {
			return nil
		}
		printLogger(a, logger)
		select {
		case <-cancel:
			return ErrWaitCancel
		case <-timer.C:
			return ErrWaitTimeOut
		case <-ticker.C:
		}
	}
}

//WaitUpgradeReady wait upgrade success
func WaitUpgradeReady(store store.Storer, a *v1.AppService, timeout time.Duration, logger event.Logger, cancel chan struct{}) error {
	if a == nil {
		return nil
	}
	if timeout < 40 {
		timeout = time.Second * 40
	}
	logger.Info(fmt.Sprintf("waiting app upgrade complete timeout %ds", int(timeout.Seconds())), map[string]string{"step": "appruntime", "status": "running"})
	logrus.Debugf("waiting app upgrade complete timeout %ds", int(timeout.Seconds()))
	ticker := time.NewTicker(timeout / 10)
	timer := time.NewTimer(timeout)
	defer ticker.Stop()
	for {
		if a.UpgradeComlete() {
			return nil
		}
		printLogger(a, logger)
		select {
		case <-cancel:
			return ErrWaitCancel
		case <-timer.C:
			return ErrWaitTimeOut
		case <-ticker.C:
		}
	}
}
func printLogger(a *v1.AppService, logger event.Logger) {
	var ready int32
	if a.GetStatefulSet() != nil {
		ready = a.GetStatefulSet().Status.ReadyReplicas
	}
	if a.GetDeployment() != nil {
		ready = a.GetDeployment().Status.ReadyReplicas
	}
	logger.Info(fmt.Sprintf("current instance(count:%d ready:%d notready:%d)", len(a.GetPods(false)), ready, int32(len(a.GetPods(false)))-ready), map[string]string{"step": "appruntime", "status": "running"})
	pods := a.GetPods(false)
	for _, pod := range pods {
		for _, con := range pod.Status.Conditions {
			if con.Status == corev1.ConditionFalse {
				// Change from Debug to Error so users can see it
				logger.Error(fmt.Sprintf("instance %s %s status is %s: %s", pod.Name, con.Type, con.Status, con.Message), map[string]string{"step": "appruntime", "status": "failure"})
			}
		}
	}
}

// checkWorkloadFailureReason checks Deployment/StatefulSet/ReplicaSet events to find why Pods are not created
func checkWorkloadFailureReason(client kubernetes.Interface, a *v1.AppService, logger event.Logger) error {
	if client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	namespace := a.TenantID
	if a.GetDeployment() != nil {
		namespace = a.GetDeployment().Namespace
	} else if a.GetStatefulSet() != nil {
		namespace = a.GetStatefulSet().Namespace
	}

	// Check Deployment events
	if deployment := a.GetDeployment(); deployment != nil {
		if err := checkDeploymentEvents(ctx, client, namespace, deployment.Name, logger); err != nil {
			return err
		}

		// Check ReplicaSet events (most important for resource quota issues)
		replicaSets, err := client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", a.ServiceAlias),
		})
		if err == nil && len(replicaSets.Items) > 0 {
			// Check the newest ReplicaSet
			for _, rs := range replicaSets.Items {
				if err := checkReplicaSetEvents(ctx, client, namespace, rs.Name, logger); err != nil {
					return err
				}
				// Check ReplicaSet conditions
				for _, cond := range rs.Status.Conditions {
					if cond.Type == "ReplicaFailure" && cond.Status == corev1.ConditionTrue {
						errMsg := translateReplicaSetFailure(cond.Reason, cond.Message)
						logger.Error(errMsg, map[string]string{"step": "appruntime", "status": "failure"})
						return fmt.Errorf(errMsg)
					}
				}
			}
		}
	}

	// Check StatefulSet events
	if statefulset := a.GetStatefulSet(); statefulset != nil {
		if err := checkStatefulSetEvents(ctx, client, namespace, statefulset.Name, logger); err != nil {
			return err
		}
	}

	return nil
}

// checkDeploymentEvents checks Deployment events for failure reasons
func checkDeploymentEvents(ctx context.Context, client kubernetes.Interface, namespace, name string, logger event.Logger) error {
	events, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Deployment", name),
	})
	if err != nil {
		return nil
	}

	for _, event := range events.Items {
		if event.Type == "Warning" && time.Since(event.LastTimestamp.Time) < time.Minute*5 {
			errMsg := translateK8sEvent(event.Reason, event.Message)
			logger.Error(fmt.Sprintf("Deployment event: %s", errMsg), map[string]string{"step": "appruntime", "status": "failure"})
			if isFailureEvent(event.Reason) {
				return fmt.Errorf(errMsg)
			}
		}
	}
	return nil
}

// checkReplicaSetEvents checks ReplicaSet events for failure reasons (THIS IS KEY FOR RESOURCE QUOTA)
func checkReplicaSetEvents(ctx context.Context, client kubernetes.Interface, namespace, name string, logger event.Logger) error {
	events, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=ReplicaSet", name),
	})
	if err != nil {
		return nil
	}

	for _, event := range events.Items {
		if event.Type == "Warning" && time.Since(event.LastTimestamp.Time) < time.Minute*5 {
			errMsg := translateK8sEvent(event.Reason, event.Message)
			logger.Error(fmt.Sprintf("ReplicaSet event: %s", errMsg), map[string]string{"step": "appruntime", "status": "failure"})
			if isFailureEvent(event.Reason) {
				return fmt.Errorf(errMsg)
			}
		}
	}
	return nil
}

// checkStatefulSetEvents checks StatefulSet events for failure reasons
func checkStatefulSetEvents(ctx context.Context, client kubernetes.Interface, namespace, name string, logger event.Logger) error {
	events, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=StatefulSet", name),
	})
	if err != nil {
		return nil
	}

	for _, event := range events.Items {
		if event.Type == "Warning" && time.Since(event.LastTimestamp.Time) < time.Minute*5 {
			errMsg := translateK8sEvent(event.Reason, event.Message)
			logger.Error(fmt.Sprintf("StatefulSet event: %s", errMsg), map[string]string{"step": "appruntime", "status": "failure"})
			if isFailureEvent(event.Reason) {
				return fmt.Errorf(errMsg)
			}
		}
	}
	return nil
}

// translateK8sEvent translates Kubernetes event reasons to user-friendly messages
func translateK8sEvent(reason, message string) string {
	switch reason {
	case "FailedCreate":
		if strings.Contains(message, "exceeded quota") || strings.Contains(message, "quota") {
			return util.Translation("Deployment failed: namespace resource quota exceeded")
		}
		if strings.Contains(message, "Insufficient cpu") {
			return util.Translation("Deployment failed: insufficient CPU resources")
		}
		if strings.Contains(message, "Insufficient memory") {
			return util.Translation("Deployment failed: insufficient memory resources")
		}
		if strings.Contains(message, "forbidden") {
			return util.Translation("Deployment failed: insufficient permissions")
		}
		return fmt.Sprintf("%s: %s", util.Translation("Deployment failed: container creation failed"), message)
	case "FailedScheduling":
		return util.Translation("Deployment failed: no nodes available for scheduling")
	case "FailedMount":
		return util.Translation("Deployment failed: persistent volume claim is pending")
	default:
		if message != "" {
			return fmt.Sprintf("%s (%s)", message, reason)
		}
		return reason
	}
}

// translateReplicaSetFailure translates ReplicaSet failure conditions
func translateReplicaSetFailure(reason, message string) string {
	if strings.Contains(message, "exceeded quota") || strings.Contains(message, "quota") {
		return util.Translation("Deployment failed: namespace resource quota exceeded")
	}
	if strings.Contains(message, "Insufficient") {
		if strings.Contains(message, "cpu") {
			return util.Translation("Deployment failed: insufficient CPU resources")
		}
		if strings.Contains(message, "memory") {
			return util.Translation("Deployment failed: insufficient memory resources")
		}
		return util.Translation("Deployment failed: insufficient storage resources")
	}
	return fmt.Sprintf("%s: %s", reason, message)
}

// isFailureEvent determines if an event reason indicates a critical failure
func isFailureEvent(reason string) bool {
	failureReasons := []string{
		"FailedCreate",
		"FailedScheduling",
		"FailedMount",
		"FailedAttachVolume",
		"FailedMapVolume",
	}
	for _, fr := range failureReasons {
		if reason == fr {
			return true
		}
	}
	return false
}
