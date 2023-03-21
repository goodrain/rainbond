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

package leader

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
)

const (
	leaseDuration = 15 * time.Second
	renewDeadline = 10 * time.Second
	retryPeriod   = 5 * time.Second
)

// RunAsLeader starts this particular external attacher after becoming a leader.
func RunAsLeader(ctx context.Context, clientset kubernetes.Interface, namespace string, identity string, lockName string, startFunc func(ctx context.Context), stopFunc func()) {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: clientset.CoreV1().Events(namespace)})
	eventRecorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: fmt.Sprintf("%s %s", lockName, string(identity))})

	rlConfig := resourcelock.ResourceLockConfig{
		Identity:      identity,
		EventRecorder: eventRecorder,
	}
	lock, err := resourcelock.New(resourcelock.ConfigMapsLeasesResourceLock, namespace, SanitizeDriverName(lockName), clientset.CoreV1(), clientset.CoordinationV1(), rlConfig)
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}

	leaderConfig := leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: leaseDuration,
		RenewDeadline: renewDeadline,
		RetryPeriod:   retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				logrus.Info("Became leader, starting")
				startFunc(ctx)
			},
			OnStoppedLeading: func() {
				logrus.Warning("Stopped leading")
				stopFunc()
			},
			OnNewLeader: func(identity string) {
				logrus.Debugf("Current leader: %s", identity)
			},
		},
	}

	leaderelection.RunOrDie(ctx, leaderConfig)
}

//SanitizeDriverName a DNS-1123 subdomain must consist of lower case alphanumeric characters,
// '-' or '.', and must start and end with an alphanumeric character
//(e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')
func SanitizeDriverName(driver string) string {
	re := regexp.MustCompile("[^a-z0-9-]")
	name := re.ReplaceAllString(driver, "-")
	if name[len(name)-1] == '-' {
		// name must not end with '-'
		name = name + "x"
	}
	return name
}
