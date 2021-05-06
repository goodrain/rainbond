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
		v1alpha1.HelmAppChartParsed,
	}
	for _, t := range types {
		if !s.helmApp.Status.IsConditionTrue(t) {
			return false
		}
	}
	return true
}
