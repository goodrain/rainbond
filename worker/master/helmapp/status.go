// RAINBOND, Application Management Platform
// Copyright (C) 2014-2021 Goodrain Co., Ltd.

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

package helmapp

import (
	"context"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// Status represents the status of helm app.
type Status struct {
	ctx            context.Context
	rainbondClient versioned.Interface
	helmApp        *v1alpha1.HelmApp
}

// NewStatus creates a new helm app status.
func NewStatus(ctx context.Context, app *v1alpha1.HelmApp, rainbondClient versioned.Interface) *Status {
	return &Status{
		ctx:            ctx,
		helmApp:        app,
		rainbondClient: rainbondClient,
	}
}

// Update updates helm app status.
func (s *Status) Update() error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ctx, cancel := context.WithTimeout(s.ctx, defaultTimeout)
		defer cancel()

		helmApp, err := s.rainbondClient.RainbondV1alpha1().HelmApps(s.helmApp.Namespace).Get(ctx, s.helmApp.Name, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "get helm app before update")
		}

		s.helmApp.Status.Phase = s.getPhase()
		s.helmApp.ResourceVersion = helmApp.ResourceVersion
		_, err = s.rainbondClient.RainbondV1alpha1().HelmApps(s.helmApp.Namespace).UpdateStatus(ctx, s.helmApp, metav1.UpdateOptions{})
		return err
	})
}

func (s *Status) getPhase() v1alpha1.HelmAppStatusPhase {
	phase := v1alpha1.HelmAppStatusPhaseDetecting
	if s.isDetected() {
		phase = v1alpha1.HelmAppStatusPhaseConfiguring
	}
	if s.helmApp.Spec.PreStatus == v1alpha1.HelmAppPreStatusConfigured {
		phase = v1alpha1.HelmAppStatusPhaseInstalling
	}
	idx, condition := s.helmApp.Status.GetCondition(v1alpha1.HelmAppInstalled)
	if idx != -1 && condition.Status == corev1.ConditionTrue {
		phase = v1alpha1.HelmAppStatusPhaseInstalled
	}
	return phase
}

func (s *Status) isDetected() bool {
	types := []v1alpha1.HelmAppConditionType{
		v1alpha1.HelmAppChartReady,
		v1alpha1.HelmAppPreInstalled,
	}
	for _, t := range types {
		if !s.helmApp.Status.IsConditionTrue(t) {
			return false
		}
	}
	return true
}
