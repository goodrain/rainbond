// RAINBOND, Application Management Platform
// Copyright (C) 2014-2026 Goodrain Co., Ltd.

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

package mirror

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// configMapName persists the last known good mirror list so a builder restart
// does not depend on the remote source being reachable.
const configMapName = "rbd-dynamic-mirrors"

const (
	fetchTimeout = 15 * time.Second
	probeTimeout = 5 * time.Second
)

// Manager holds the dynamically refreshed docker.io mirror list. All methods
// are safe for concurrent use; Mirrors is additionally nil-receiver safe so
// call sites do not need to care whether the feature was initialised.
type Manager struct {
	cfg       Config
	kube      kubernetes.Interface
	namespace string

	mu      sync.RWMutex
	mirrors []string
}

var (
	defaultManager *Manager
	defaultMu      sync.RWMutex
)

// New creates a Manager. kube may be nil in unit tests that skip persistence.
func New(cfg Config, kube kubernetes.Interface, namespace string) *Manager {
	return &Manager{cfg: cfg, kube: kube, namespace: namespace}
}

// Init creates the default Manager from the environment and starts its refresh
// loop. It is a no-op when the feature is disabled.
func Init(ctx context.Context, getenv func(string) string, kube kubernetes.Interface, namespace string) {
	cfg := LoadConfig(getenv)
	if !cfg.Enabled {
		logrus.Info("dynamic registry mirrors disabled")
		return
	}
	m := New(cfg, kube, namespace)
	defaultMu.Lock()
	defaultManager = m
	defaultMu.Unlock()
	go m.Start(ctx)
}

// Default returns the manager created by Init, or nil when the feature is
// disabled or not initialised. The returned value is safe to use either way.
func Default() *Manager {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultManager
}

// Mirrors returns a copy of the current mirror list, ordered by probe latency.
func (m *Manager) Mirrors() []string {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.mirrors) == 0 {
		return nil
	}
	result := make([]string, len(m.mirrors))
	copy(result, m.mirrors)
	return result
}

// Start restores the persisted list, refreshes immediately and then keeps
// refreshing on the configured interval until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) {
	if !m.cfg.Enabled {
		return
	}
	m.restore(ctx)
	if err := m.Refresh(ctx); err != nil {
		logrus.Warnf("initial dynamic mirror refresh failure: %v", err)
	}
	ticker := time.NewTicker(m.cfg.RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.Refresh(ctx); err != nil {
				logrus.Warnf("dynamic mirror refresh failure: %v", err)
			}
		}
	}
}

// Refresh fetches candidates, probes them and stores the fastest MaxCount
// alive mirrors. A fetch failure returns an error and keeps the previous list;
// an empty probe result is not an error and clears the list, so dead mirrors
// never reach the build path.
func (m *Manager) Refresh(ctx context.Context) error {
	if !m.cfg.Enabled {
		return nil
	}
	candidates, err := FetchCandidates(ctx, m.cfg.SourceURLs, fetchTimeout)
	if err != nil {
		return fmt.Errorf("fetch mirror candidates: %w", err)
	}
	alive := Probe(ctx, candidates, probeTimeout)
	if len(alive) > m.cfg.MaxCount {
		alive = alive[:m.cfg.MaxCount]
	}
	m.setMirrors(ctx, alive)
	logrus.Infof("dynamic registry mirrors refreshed: %d candidates, using %v", len(candidates), alive)
	return nil
}

func (m *Manager) setMirrors(ctx context.Context, mirrors []string) {
	m.mu.Lock()
	m.mirrors = mirrors
	m.mu.Unlock()
	if err := m.persist(ctx, mirrors); err != nil {
		logrus.Warnf("persist dynamic mirrors failure: %v", err)
	}
}

// restore loads the last persisted list so mirrors are usable right after a
// builder restart, before the first remote fetch completes.
func (m *Manager) restore(ctx context.Context) {
	if m.kube == nil {
		return
	}
	cm, err := m.kube.CoreV1().ConfigMaps(m.namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		if !k8serror.IsNotFound(err) {
			logrus.Warnf("restore dynamic mirrors failure: %v", err)
		}
		return
	}
	var mirrors []string
	if err := json.Unmarshal([]byte(cm.Data["mirrors"]), &mirrors); err != nil {
		logrus.Warnf("restore dynamic mirrors: invalid persisted payload: %v", err)
		return
	}
	if len(mirrors) == 0 {
		return
	}
	m.mu.Lock()
	m.mirrors = mirrors
	m.mu.Unlock()
	logrus.Infof("dynamic registry mirrors restored from configmap: %v", mirrors)
}

func (m *Manager) persist(ctx context.Context, mirrors []string) error {
	if m.kube == nil {
		return nil
	}
	payload, err := json.Marshal(mirrors)
	if err != nil {
		return fmt.Errorf("marshal mirrors: %w", err)
	}
	data := map[string]string{
		"mirrors":    string(payload),
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	cm, err := m.kube.CoreV1().ConfigMaps(m.namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if k8serror.IsNotFound(err) {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: configMapName, Namespace: m.namespace},
			Data:       data,
		}
		_, err = m.kube.CoreV1().ConfigMaps(m.namespace).Create(ctx, cm, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	cm.Data = data
	_, err = m.kube.CoreV1().ConfigMaps(m.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}
