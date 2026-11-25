// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package clean

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/goodrain/rainbond-operator/util/constants"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/config/configs"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	utils "github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// Runner performs cleanup through the coordinated admission and execution path.
// It must never fall back to uncoordinated registry or filesystem cleanup.
type Runner func(context.Context, string, uint) error

// Manager schedules coordinated cleanup without directly deleting artifacts.
type Manager struct {
	imageClient sources.ImageClient
	ctx         context.Context
	cancel      context.CancelFunc
	keepCount   uint
	schedule    dailySchedule
	runner      Runner
	now         func() time.Time
	wait        func(context.Context, time.Duration) error
	startMu     sync.Mutex
	started     bool
}

// CreateCleanManager creates a daily cleanup scheduler. The optional runner
// preserves constructor compatibility; Start rejects a missing runner.
func CreateCleanManager(imageClient sources.ImageClient, runners ...Runner) (*Manager, error) {
	config := configs.Default().ChaosConfig
	if config.KeepCount <= 0 {
		return nil, fmt.Errorf("cleanup keep-count must be positive")
	}
	schedule, err := newDailySchedule(config.CleanTime, config.CleanTimeZone)
	if err != nil {
		return nil, err
	}
	if len(runners) > 1 {
		return nil, fmt.Errorf("cleanup accepts at most one coordinated runner")
	}
	var runner Runner
	if len(runners) == 1 {
		runner = runners[0]
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{imageClient: imageClient, ctx: ctx, cancel: cancel,
		keepCount: uint(config.KeepCount), schedule: schedule, runner: runner,
		now: time.Now, wait: waitForCleanupTime}, nil
}

func waitForCleanupTime(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Start launches the injected runner once at startup and then at daily times.
// Controlled cleanup errors are warnings, not fatal errors for Chaos.
func (t *Manager) Start(_ chan error) error {
	t.startMu.Lock()
	defer t.startMu.Unlock()
	if t.runner == nil {
		return fmt.Errorf("coordinated cleanup runner is required")
	}
	if t.ctx == nil || t.schedule.location == nil || t.now == nil || t.wait == nil {
		return fmt.Errorf("cleanup manager is not initialized")
	}
	if err := t.ctx.Err(); err != nil {
		return err
	}
	if t.started {
		return fmt.Errorf("cleanup manager is already started")
	}
	t.started = true
	logrus.Info("coordinated cleanup scheduler is starting")
	go func() {
		startup := true
		runDaily(t.ctx, t.schedule, t.now, t.wait, func(ctx context.Context, scheduled time.Time) {
			trigger := "startup"
			if !startup {
				trigger = "daily:" + scheduled.Format(time.RFC3339)
			}
			startup = false
			t.runTrigger(ctx, trigger)
		})
	}()
	return nil
}

// runTrigger retries only known contention, retaining the ledger trigger identity.
func (t *Manager) runTrigger(ctx context.Context, trigger string) {
	for ctx.Err() == nil {
		err := t.runner(ctx, trigger, t.keepCount)
		if err == nil || ctx.Err() != nil {
			return
		}
		if !errors.Is(err, guard.ErrCoordinationBusy) {
			logrus.WithError(err).Warn("coordinated cleanup skipped or failed")
			return
		}
		logrus.Warn("coordinated cleanup busy; postponing the same trigger for one minute")
		if err := t.wait(ctx, time.Minute); err != nil {
			return
		}
	}
}

// Stop cancels the scheduler and any context-aware coordinated cleanup run.
func (t *Manager) Stop() error {
	logrus.Info("coordinated cleanup scheduler is stopping")
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

// PodExecCmd registry garbage-collect
func (t *Manager) PodExecCmd(config *rest.Config, clientset kubernetes.Interface, podName string, cmd []string) (stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"name": podName}}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	}
	pods, err := clientset.CoreV1().Pods(utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)).List(context.TODO(), listOptions)
	if err != nil {
		return stdout, stderr, err
	}

	for _, pod := range pods.Items {
		req := clientset.CoreV1().RESTClient().Post().
			Namespace(utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)).
			Resource("pods").
			Name(pod.Name).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Command: cmd,
				Stdin:   false,
				Stdout:  true,
				Stderr:  true,
				TTY:     false,
			}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
		if err != nil {
			return stdout, stderr, err
		}
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  nil,
			Stdout: &stdout,
			Stderr: &stderr,
			Tty:    false,
		})
		if err != nil {
			return stdout, stderr, err
		}
		return stdout, stderr, nil
	}
	return stdout, stderr, nil
}
