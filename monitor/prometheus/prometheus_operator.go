// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltd.

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

package prometheus

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	mv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	externalversions "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

//PrometheusOperator service monitor
type PrometheusOperator struct {
	ctx        context.Context
	Prometheus *Manager
	smInf      cache.SharedIndexInformer
	prInf      cache.SharedIndexInformer
	queue      workqueue.RateLimitingInterface
}

//NewPrometheusOperator new sm controller
func NewPrometheusOperator(ctx context.Context, smFactory externalversions.SharedInformerFactory, pm *Manager) (*PrometheusOperator, error) {
	var smc PrometheusOperator
	smc.ctx = ctx
	smc.smInf = smFactory.Monitoring().V1().ServiceMonitors().Informer()
	smc.prInf = smFactory.Monitoring().V1().PrometheusRules().Informer()
	smc.smInf.AddEventHandlerWithResyncPeriod(&smc, time.Second*30)
	smc.prInf.AddEventHandlerWithResyncPeriod(&smc, time.Second*30)
	smc.Prometheus = pm
	smc.queue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "sm-monitor")
	return &smc, nil
}

//Run run controller
func (s *PrometheusOperator) Run(stopCh <-chan struct{}) {
	go s.worker(s.ctx)
	go s.smInf.Run(stopCh)
	go s.prInf.Run(stopCh)
	cache.WaitForCacheSync(stopCh, s.smInf.HasSynced, s.prInf.HasSynced)
	logrus.Info("prometheus operator start success")
}

func (s *PrometheusOperator) Stop() {
	s.queue.ShutDown()
}

//OnAdd sm add
func (s *PrometheusOperator) OnAdd(obj interface{}) {
	s.enqueue(obj)
}

//OnUpdate sm update
func (s *PrometheusOperator) OnUpdate(oldObj, newObj interface{}) {
	s.enqueue(newObj)
}

//OnDelete sm delete
func (s *PrometheusOperator) OnDelete(obj interface{}) {
	s.enqueue(obj)
}
func (s *PrometheusOperator) enqueue(obj interface{}) {
	if obj == nil {
		return
	}
	key, ok := obj.(string)
	if !ok {
		key, ok = s.keyFunc(obj)
		if !ok {
			return
		}
	}
	s.queue.Add(key)
}

func (s *PrometheusOperator) keyFunc(obj interface{}) (string, bool) {
	//namespace and name
	k, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logrus.Errorf("creating key failed %s", err.Error())
		return k, false
	}
	return k, true
}

func (s *PrometheusOperator) worker(ctx context.Context) {
	for s.processNextWorkItem(ctx) {
	}
}
func (s *PrometheusOperator) processNextWorkItem(ctx context.Context) bool {
	key, quit := s.queue.Get()
	if quit {
		return false
	}
	defer s.queue.Done(key)
	s.sync()
	s.queue.Forget(key)
	return true
}

func (s *PrometheusOperator) sync() {
	logrus.Debug("start sync prometheus rule config to prometheus config")
	prList := s.prInf.GetStore().List()
	var prometheusRules []*mv1.PrometheusRule
	for j := range prList {
		if pr, ok := prList[j].(*mv1.PrometheusRule); ok && pr != nil {
			prometheusRules = append(prometheusRules, pr)
		}
	}
	logrus.Debug("start sync service monitor config to prometheus config")
	smList := s.smInf.GetStore().List()
	var scrapes []*ScrapeConfig
	sMonIdentifiers := make([]string, len(smList))
	sMons := make(map[string]*mv1.ServiceMonitor, len(smList))
	i := 0
	for j := range smList {
		if sm, ok := smList[j].(*mv1.ServiceMonitor); ok && sm != nil {
			sMonIdentifiers[i] = sm.GetNamespace() + "/" + sm.GetName()
			sMons[sm.GetNamespace()+"/"+sm.GetName()] = sm
			i++
		}
	}

	// Sorting ensures, that we always generate the config in the same order.
	sort.Strings(sMonIdentifiers)
	for _, name := range sMonIdentifiers {
		for i, end := range sMons[name].Spec.Endpoints {
			scrape := s.createScrapeBySM(sMons[name], end, i)
			scrapes = append(scrapes, scrape)
		}
	}
	s.Prometheus.UpdateScrapeAndRule(scrapes, prometheusRules)
	logrus.Debugf("success sync prometheus configs , scrapes length: %d, rule length: %d", len(scrapes), len(prometheusRules))
}

func (s *PrometheusOperator) createScrapeBySM(sm *mv1.ServiceMonitor, ep mv1.Endpoint, i int) *ScrapeConfig {
	var sc = ScrapeConfig{
		JobName: fmt.Sprintf("%s/%s/%d", sm.Namespace, sm.Name, i),
		ServiceDiscoveryConfig: ServiceDiscoveryConfig{
			KubernetesSDConfigs: []*SDConfig{
				{
					Role: RoleEndpoint,
					NamespaceDiscovery: NamespaceDiscovery{
						Names: []string{sm.Namespace},
					},
					Selectors: []SelectorConfig{
						{
							Role: RoleEndpoint,
						},
					},
				},
			},
		},
	}

	if ep.Interval != "" {
		sc.ScrapeInterval = parseDuration(ep.Interval, time.Second*15)
	}
	if ep.ScrapeTimeout != "" {
		sc.ScrapeTimeout = parseDuration(ep.ScrapeTimeout, time.Second*10)
	}
	if ep.Path != "" {
		sc.MetricsPath = ep.Path
	}
	if ep.ProxyURL != nil && *ep.ProxyURL != "" {
		purl, _ := url.Parse(*ep.ProxyURL)
		if purl != nil {
			sc.HTTPClientConfig.ProxyURL = URL{purl}
		}
	}
	if ep.Params != nil {
		sc.Params = ep.Params
	}
	if ep.Scheme != "" {
		sc.Scheme = ep.Scheme
	}
	if ep.TLSConfig != nil {
		sc.HTTPClientConfig.TLSConfig = TLSConfig{
			CAFile:   ep.TLSConfig.CAFile,
			CertFile: ep.TLSConfig.CertFile,
			KeyFile:  ep.TLSConfig.KeyFile,
		}
	}
	if ep.BearerTokenFile != "" {
		sc.HTTPClientConfig.BearerTokenFile = ep.BearerTokenFile
	}
	var labelKeys []string
	for k := range sm.Spec.Selector.MatchLabels {
		labelKeys = append(labelKeys, k)
	}
	sort.Strings(labelKeys)

	for _, k := range labelKeys {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
			Action:       "keep",
			SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_service_label_" + sanitizeLabelName(k))},
			Regex:        MustNewRegexp(sm.Spec.Selector.MatchLabels[k]),
		})
	}

	// Set based label matching. We have to map the valid relations
	// `In`, `NotIn`, `Exists`, and `DoesNotExist`, into relabeling rules.
	for _, exp := range sm.Spec.Selector.MatchExpressions {
		switch exp.Operator {
		case metav1.LabelSelectorOpIn:
			sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
				Action:       "keep",
				SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_service_label_" + sanitizeLabelName(exp.Key))},
				Regex:        MustNewRegexp(strings.Join(exp.Values, "|")),
			})
		case metav1.LabelSelectorOpNotIn:
			sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
				Action:       "drop",
				SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_service_label_" + sanitizeLabelName(exp.Key))},
				Regex:        MustNewRegexp(strings.Join(exp.Values, "|")),
			})
		case metav1.LabelSelectorOpExists:
			sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
				Action:       "keep",
				SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_service_label_" + sanitizeLabelName(exp.Key))},
				Regex:        MustNewRegexp(".+"),
			})
		case metav1.LabelSelectorOpDoesNotExist:
			sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
				Action:       "drop",
				SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_service_label_" + sanitizeLabelName(exp.Key))},
				Regex:        MustNewRegexp(".+"),
			})
		}
	}
	// Filter targets based on correct port for the endpoint.
	if ep.Port != "" {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
			Action:       "keep",
			SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_endpoint_port_name")},
			Regex:        MustNewRegexp(ep.Port),
		})
	} else if ep.TargetPort != nil {
		if ep.TargetPort.StrVal != "" {
			sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
				Action:       "keep",
				SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_pod_container_port_name")},
				Regex:        MustNewRegexp(ep.TargetPort.String()),
			})
		} else if ep.TargetPort.IntVal != 0 {
			sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
				Action:       "keep",
				SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_pod_container_port_number")},
				Regex:        MustNewRegexp(ep.TargetPort.String()),
			})
		}
	}
	sc.RelabelConfigs = append(sc.RelabelConfigs, []*RelabelConfig{
		{ // Relabel node labels for pre v2.3 meta labels

			SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_endpoint_address_target_kind"), model.LabelName("__meta_kubernetes_endpoint_address_target_name")},
			Regex:        MustNewRegexp("Node;(.*)"),
			Replacement:  "${1}",
			TargetLabel:  "node",
			Separator:    ";",
		},
		{ // Relabel pod labels for >=v2.3 meta labels
			SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_endpoint_address_target_kind"), model.LabelName("__meta_kubernetes_endpoint_address_target_name")},
			Regex:        MustNewRegexp("Pod;(.*)"),
			Replacement:  "${1}",
			TargetLabel:  "pod",
			Separator:    ";",
		},
		{
			SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_namespace")},
			TargetLabel:  "namespace",
		},
		{
			SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_service_name")},
			TargetLabel:  "service",
		},
		{
			SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_pod_name")},
			TargetLabel:  "pod",
		},
		{
			SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_pod_container_name")},
			TargetLabel:  "container",
		},
	}...)
	// Relabel targetLabels from Service onto target.
	for _, l := range sm.Spec.TargetLabels {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
			SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_service_label_" + sanitizeLabelName(l))},
			TargetLabel:  sanitizeLabelName(l),
			Regex:        MustNewRegexp("(.+)"),
			Replacement:  "${1}",
		})
	}

	for _, l := range sm.Spec.PodTargetLabels {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
			SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_pod_label_" + sanitizeLabelName(l))},
			TargetLabel:  sanitizeLabelName(l),
			Regex:        MustNewRegexp("(.+)"),
			Replacement:  "${1}",
		})
	}

	// By default, generate a safe job name from the service name.  We also keep
	// this around if a jobLabel is set in case the targets don't actually have a
	// value for it. A single service may potentially have multiple metrics
	// endpoints, therefore the endpoints labels is filled with the ports name or
	// as a fallback the port number.

	sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
		SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_service_name")},
		TargetLabel:  "job",
		Replacement:  "${1}",
	})
	if sm.Spec.JobLabel != "" {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
			SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_service_label_" + sanitizeLabelName(sm.Spec.JobLabel))},
			TargetLabel:  "job",
			Replacement:  "${1}",
			Regex:        MustNewRegexp("(.+)"),
		})
	}

	if ep.Port != "" {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
			TargetLabel: "endpoint",
			Replacement: ep.Port,
		})
	} else if ep.TargetPort != nil && ep.TargetPort.String() != "" {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
			TargetLabel: "endpoint",
			Replacement: ep.TargetPort.String(),
		})
	}

	if ep.RelabelConfigs != nil {
		for _, c := range ep.RelabelConfigs {
			sc.RelabelConfigs = append(sc.RelabelConfigs, &RelabelConfig{
				SourceLabels: func() (re model.LabelNames) {
					for _, l := range c.SourceLabels {
						re = append(re, model.LabelName(l))
					}
					return
				}(),
				Separator:   c.Separator,
				TargetLabel: c.TargetLabel,
				Regex:       MustNewRegexp(c.Regex),
				Modulus:     c.Modulus,
				Replacement: c.Replacement,
				Action:      RelabelAction(c.Action),
			})
		}
	}
	return &sc
}

func parseDuration(source string, def time.Duration) model.Duration {
	d, err := time.ParseDuration(source)
	if err != nil {
		return model.Duration(def)
	}
	return model.Duration(d)
}

var (
	invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

func sanitizeLabelName(name string) string {
	return invalidLabelCharRE.ReplaceAllString(name, "_")
}
