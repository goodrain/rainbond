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

package handler

import (
	"context"
	"fmt"
	apisixversioned "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/component/mq"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm/controller"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1beta1"
	"sort"
	"strconv"
	"strings"
)

// GatewayAction -
type GatewayAction struct {
	dbmanager     db.Manager
	mqclient      client.MQClient
	gatewayClient *gateway.GatewayV1beta1Client
	kubeClient    kubernetes.Interface
	kubeClientset *kubernetes.Clientset
	config        *rest.Config
	apisixClient  *apisixversioned.Clientset
}

// CreateGatewayManager creates gateway manager.
func CreateGatewayManager() *GatewayAction {
	return &GatewayAction{
		dbmanager:     db.GetManager(),
		mqclient:      mq.Default().MqClient,
		gatewayClient: k8s.Default().GatewayClient,
		kubeClient:    k8s.Default().Clientset,
		kubeClientset: k8s.Default().Clientset,
		config:        k8s.Default().RestConfig,
		apisixClient:  k8s.Default().ApiSixClient,
	}
}

// GetClient -
func (g *GatewayAction) GetClient() *apisixversioned.Clientset {
	return g.apisixClient
}

// GetK8sClient -
func (g *GatewayAction) GetK8sClient() kubernetes.Interface {
	return g.kubeClient
}

// CreateCert -
func (g *GatewayAction) CreateCert(namespace, domain string) error {
	secretName := strings.Replace(domain, ".", "-", -1)

	// Generate self-signed certificate
	cert, certKey, err := generateSelfSignedCertificate(domain)
	if err != nil {
		logrus.Errorf("Error generating self-signed certificate: %v", err)
		return err
	}

	// Create Kubernetes Secret
	return createK8sSecret(g.kubeClientset, namespace, secretName, cert, certKey)
}

// BatchGetGatewayHTTPRoute batch get gateway http route
func (g *GatewayAction) BatchGetGatewayHTTPRoute(namespace, appID string) ([]*apimodel.GatewayHTTPRouteConcise, error) {
	var httpRoutes []v1beta1.HTTPRoute
	if appID != "" {
		gatewayHTTPRoutes, err := g.gatewayClient.HTTPRoutes(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: "app_id=" + appID})
		if err != nil {
			logrus.Errorf("list http route by app_id = %v failure: %v", appID, err)
			return nil, err
		}
		httpRoutes = gatewayHTTPRoutes.Items
	} else {
		gatewayHTTPRoutes, err := g.gatewayClient.HTTPRoutes(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("list http route failure: %v", err)
			return nil, err
		}
		httpRoutes = gatewayHTTPRoutes.Items
	}
	var HTTPRouteConcise []*apimodel.GatewayHTTPRouteConcise
	for _, httpRoute := range httpRoutes {
		var gatewayName string
		gatewayNamespace := namespace
		if httpRoute.Spec.ParentRefs != nil {
			gatewayName = string(httpRoute.Spec.ParentRefs[0].Name)
			if httpRoute.Spec.ParentRefs[0].Namespace != nil {
				gatewayNamespace = string(*httpRoute.Spec.ParentRefs[0].Namespace)
			}
		}
		var hosts []string
		if httpRoute.Spec.Hostnames != nil {
			for _, hostname := range httpRoute.Spec.Hostnames {
				hosts = append(hosts, string(hostname))
			}
		}
		var id string
		if httpRoute.Labels != nil {
			id = httpRoute.Labels["app_id"]
		}
		HTTPRouteConcise = append(HTTPRouteConcise, &apimodel.GatewayHTTPRouteConcise{
			Name:             httpRoute.Name,
			Hosts:            hosts,
			GatewayName:      gatewayName,
			GatewayNamespace: gatewayNamespace,
			AppID:            id,
		})
	}
	return HTTPRouteConcise, nil
}

// AddGatewayCertificate create gateway certificate
func (g *GatewayAction) AddGatewayCertificate(req *apimodel.GatewayCertificate) error {
	_, err := g.kubeClient.CoreV1().Secrets(req.Namespace).Create(context.Background(), &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       apimodel.Secret,
			APIVersion: controller.APIVersionSecret,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Data: map[string][]byte{
			"tls.crt": []byte(req.Certificate),
			"tls.key": []byte(req.PrivateKey),
		},
		Type: corev1.SecretTypeTLS,
	}, metav1.CreateOptions{})
	if err != nil {
		logrus.Errorf("add gateway certificate secret failure: %v", err)
		return err
	}
	return nil
}

// UpdateGatewayCertificate update gateway certificate
func (g *GatewayAction) UpdateGatewayCertificate(req *apimodel.GatewayCertificate) error {
	secret, err := g.kubeClient.CoreV1().Secrets(req.Namespace).Get(context.Background(), req.Name, metav1.GetOptions{})
	if err != nil {
		if k8serror.IsNotFound(err) {
			secret, err = g.kubeClient.CoreV1().Secrets(req.Namespace).Create(context.Background(), &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       apimodel.Secret,
					APIVersion: controller.APIVersionSecret,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      req.Name,
					Namespace: req.Namespace,
				},
				Data: map[string][]byte{
					"tls.crt": []byte(req.Certificate),
					"tls.key": []byte(req.PrivateKey),
				},
				Type: corev1.SecretTypeTLS,
			}, metav1.CreateOptions{})
			if err != nil {
				logrus.Errorf("get gateway certificate secret, add failure: %v", err)
				return err
			}
			return nil
		}
		logrus.Errorf("update gateway certificate secret, get failure: %v", err)
		return err
	}
	certificate := make(map[string][]byte)
	certificate["tls.crt"] = []byte(req.Certificate)
	certificate["tls.key"] = []byte(req.PrivateKey)
	secret.Data = certificate
	secret, err = g.kubeClient.CoreV1().Secrets(req.Namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("update gateway certificate secret, update failure: %v", err)
		return err
	}
	return nil
}

// DeleteGatewayCertificate delete gateway certificate
func (g *GatewayAction) DeleteGatewayCertificate(name, namespace string) error {
	err := g.kubeClient.CoreV1().Secrets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		logrus.Errorf("delete gateway certificate secret failure: %v", err)
		return err
	}
	return nil
}

func handleGatewayRules(req *apimodel.GatewayHTTPRouteStruct) []v1beta1.HTTPRouteRule {
	var rules []v1beta1.HTTPRouteRule
	for _, rule := range req.Rules {
		var (
			backendRefs []v1beta1.HTTPBackendRef
			matches     []v1beta1.HTTPRouteMatch
			filters     []v1beta1.HTTPRouteFilter
		)
		if rule.MatchesRules != nil {
			for _, match := range rule.MatchesRules {
				var httpRouteMatch v1beta1.HTTPRouteMatch
				if path := match.Path; path != nil {
					pathType := v1beta1.PathMatchType(path.Type)
					value := path.Value
					httpRouteMatch.Path = &v1beta1.HTTPPathMatch{
						Type:  &pathType,
						Value: &value,
					}
				}
				if headers := match.Headers; headers != nil {
					for _, header := range headers {
						headerType := v1beta1.HeaderMatchType(header.Type)
						httpRouteMatch.Headers = append(httpRouteMatch.Headers, v1beta1.HTTPHeaderMatch{
							Name:  v1beta1.HTTPHeaderName(header.Name),
							Type:  &headerType,
							Value: header.Value,
						})
					}
				}
				matches = append(matches, httpRouteMatch)
			}
		}
		if rule.BackendRefsRules != nil {
			for _, backendRef := range rule.BackendRefsRules {
				var group v1beta1.Group
				if backendRef.Kind == apimodel.HTTPRoute {
					group = v1beta1.GroupName
				}
				kind := v1beta1.Kind(backendRef.Kind)
				namespace := v1beta1.Namespace(backendRef.Namespace)
				var port *v1beta1.PortNumber
				if backendRef.Port != 0 {
					p := v1beta1.PortNumber(backendRef.Port)
					port = &p
				}
				weight := int32(backendRef.Weight)
				backendRefs = append(backendRefs, v1beta1.HTTPBackendRef{
					BackendRef: v1beta1.BackendRef{
						BackendObjectReference: v1beta1.BackendObjectReference{
							Group:     &group,
							Kind:      &kind,
							Name:      v1beta1.ObjectName(backendRef.Name),
							Namespace: &namespace,
							Port:      port,
						},
						Weight: &weight,
					},
				})
			}
		}
		if rule.FiltersRules != nil {
			for _, filter := range rule.FiltersRules {
				var httpRoutefilter v1beta1.HTTPRouteFilter
				if filter.RequestHeaderModifier != nil {
					var setHTTPHeader []v1beta1.HTTPHeader
					var addHTTPHeader []v1beta1.HTTPHeader
					if filter.RequestHeaderModifier.Set != nil {
						for _, set := range filter.RequestHeaderModifier.Set {
							setHTTPHeader = append(setHTTPHeader, v1beta1.HTTPHeader{
								Name:  v1beta1.HTTPHeaderName(set.Name),
								Value: set.Value,
							})
						}
					}
					if filter.RequestHeaderModifier.Add != nil {
						for _, add := range filter.RequestHeaderModifier.Add {
							addHTTPHeader = append(addHTTPHeader, v1beta1.HTTPHeader{
								Name:  v1beta1.HTTPHeaderName(add.Name),
								Value: add.Value,
							})
						}
					}
					httpRoutefilter.RequestHeaderModifier = &v1beta1.HTTPHeaderFilter{
						Set:    setHTTPHeader,
						Add:    addHTTPHeader,
						Remove: filter.RequestHeaderModifier.Remove,
					}
				}
				if filter.RequestRedirect != nil {
					scheme := filter.RequestRedirect.Scheme
					hostname := v1beta1.PreciseHostname(filter.RequestRedirect.Hostname)
					var port *v1beta1.PortNumber
					var sc *int
					if v1beta1.PortNumber(filter.RequestRedirect.Port) != 0 {
						p := v1beta1.PortNumber(filter.RequestRedirect.Port)
						port = &p
					}
					if filter.RequestRedirect.StatusCode != 0 {
						s := filter.RequestRedirect.StatusCode
						sc = &s
					}
					httpRoutefilter.RequestRedirect = &v1beta1.HTTPRequestRedirectFilter{
						Scheme:     &scheme,
						Hostname:   &hostname,
						Port:       port,
						StatusCode: sc,
					}
				}
				httpRoutefilter.Type = v1beta1.HTTPRouteFilterType(filter.Type)
				filters = append(filters, httpRoutefilter)
			}
		}
		rule := v1beta1.HTTPRouteRule{
			Matches:     matches,
			BackendRefs: backendRefs,
			Filters:     filters,
		}
		rules = append(rules, rule)
	}
	return rules
}

// AddGatewayHTTPRoute create gateway http route
func (g *GatewayAction) AddGatewayHTTPRoute(req *apimodel.GatewayHTTPRouteStruct) (*model.K8sResource, error) {
	gatewayNamespace := v1beta1.Namespace(req.GatewayNamespace)
	var hosts []v1beta1.Hostname
	for _, host := range req.Hosts {
		hosts = append(hosts, v1beta1.Hostname(host))
	}
	rules := handleGatewayRules(req)
	labels := make(map[string]string)
	labels["app_id"] = req.AppID
	var sectionName *v1beta1.SectionName
	if req.SectionName != "" {
		sn := v1beta1.SectionName(req.SectionName)
		sectionName = &sn
	}
	httpRoute, err := g.gatewayClient.HTTPRoutes(req.Namespace).Create(context.Background(), &v1beta1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       apimodel.HTTPRoute,
			APIVersion: controller.APIVersionHTTPRoute,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
			Labels:    labels,
		},
		Spec: v1beta1.HTTPRouteSpec{
			CommonRouteSpec: v1beta1.CommonRouteSpec{
				ParentRefs: []v1beta1.ParentReference{{
					Name:        v1beta1.ObjectName(req.GatewayName),
					Namespace:   &gatewayNamespace,
					SectionName: sectionName,
				}},
			},
			Hostnames: hosts,
			Rules:     rules,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		logrus.Errorf("create gateway http route %v failure: %v", req.Name, err)
		return nil, err
	}
	httpRoute.Kind = apimodel.HTTPRoute
	httpRoute.APIVersion = controller.APIVersionHTTPRoute
	httpRouteYaml, err := ObjectToJSONORYaml("yaml", &httpRoute)
	if err != nil {
		logrus.Errorf("create gateway http route object to yaml failure: %v", err)
		return nil, err
	}
	k8sresource := []*model.K8sResource{{
		AppID:         req.AppID,
		Name:          req.Name,
		Kind:          apimodel.HTTPRoute,
		Content:       httpRouteYaml,
		ErrorOverview: "创建成功",
		State:         apimodel.CreateSuccess,
	}}
	err = db.GetManager().K8sResourceDao().CreateK8sResource(k8sresource)
	if err != nil {
		logrus.Errorf("database operation gateway http route create k8s resource failure: %v", err)
		return nil, err
	}
	return k8sresource[0], nil
}

// GetGatewayHTTPRoute get gateway http route
func (g *GatewayAction) GetGatewayHTTPRoute(name, namespace string) (*apimodel.GatewayHTTPRouteStruct, error) {
	var req apimodel.GatewayHTTPRouteStruct
	route, err := g.gatewayClient.HTTPRoutes(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("get gateway route failure: %v", err)
		return nil, err
	}
	var gatewayName, gatewayNamespace, sectionName string
	if route.Spec.ParentRefs != nil {
		gatewayName = string(route.Spec.ParentRefs[0].Name)
		if route.Spec.ParentRefs[0].Namespace != nil {
			gatewayNamespace = string(*route.Spec.ParentRefs[0].Namespace)
		}
		if route.Spec.ParentRefs[0].SectionName != nil {
			sectionName = string(*route.Spec.ParentRefs[0].SectionName)
		}
	}
	var hosts []string
	if route.Spec.Hostnames != nil {
		for _, hostname := range route.Spec.Hostnames {
			hosts = append(hosts, string(hostname))
		}
	}
	var rules []*apimodel.Rules
	for _, rule := range route.Spec.Rules {
		var matchesRules []*apimodel.MatchesRule
		var backendRefsRules []*apimodel.BackendRefsRule
		var filtersRules []*apimodel.FiltersRule
		if rule.Matches != nil {
			for _, match := range rule.Matches {
				var path apimodel.MatchesRulePath
				var headers []*apimodel.MatchesRuleHeader
				if match.Headers != nil {
					for _, header := range match.Headers {
						var ty string
						if header.Type != nil {
							ty = string(*header.Type)
						}
						headers = append(headers, &apimodel.MatchesRuleHeader{
							Name:  string(header.Name),
							Type:  ty,
							Value: header.Value,
						})
					}

				}
				if match.Path != nil {
					var ty, value string
					if match.Path.Type != nil {
						ty = string(*match.Path.Type)
					}
					if match.Path.Value != nil {
						value = *match.Path.Value
					}
					path.Type = ty
					path.Value = value
				}
				matchesRules = append(matchesRules, &apimodel.MatchesRule{
					Path:    &path,
					Headers: headers,
				})
			}
		}
		if rule.Filters != nil {
			for _, filter := range rule.Filters {
				var filterRule apimodel.FiltersRule
				filterRule.Type = string(filter.Type)
				if filter.RequestHeaderModifier != nil {
					var setHTTPHeader []*apimodel.HTTPHeader
					var addHTTPHeader []*apimodel.HTTPHeader
					var remove []string
					if filter.RequestHeaderModifier.Add != nil {
						for _, add := range filter.RequestHeaderModifier.Add {
							addHTTPHeader = append(addHTTPHeader, &apimodel.HTTPHeader{
								Name:  string(add.Name),
								Value: add.Value,
							})
						}
					}
					if filter.RequestHeaderModifier.Set != nil {
						for _, set := range filter.RequestHeaderModifier.Set {
							setHTTPHeader = append(setHTTPHeader, &apimodel.HTTPHeader{
								Name:  string(set.Name),
								Value: set.Value,
							})
						}
					}
					if filter.RequestHeaderModifier.Remove != nil {
						for _, re := range filter.RequestHeaderModifier.Remove {
							remove = append(remove, re)
						}
					}
					filterRule.RequestHeaderModifier = &apimodel.HTTPHeaderFilter{
						Set:    setHTTPHeader,
						Add:    addHTTPHeader,
						Remove: remove,
					}
				}
				if filter.RequestRedirect != nil {
					var hostname, scheme string
					var statusCode, port int
					if filter.RequestRedirect.Hostname != nil {
						hostname = string(*filter.RequestRedirect.Hostname)
					}
					if filter.RequestRedirect.Scheme != nil {
						scheme = *filter.RequestRedirect.Scheme
					}
					if filter.RequestRedirect.StatusCode != nil {
						statusCode = *filter.RequestRedirect.StatusCode
					}
					if filter.RequestRedirect.Port != nil {
						port = int(*filter.RequestRedirect.Port)
					}
					filterRule.RequestRedirect = &apimodel.HTTPRequestRedirectFilter{
						Scheme:     scheme,
						Hostname:   hostname,
						Port:       port,
						StatusCode: statusCode,
					}
				}
				filtersRules = append(filtersRules, &filterRule)
			}
		}
		if rule.BackendRefs != nil {
			for _, backendRef := range rule.BackendRefs {
				weight := 100
				kind := apimodel.Service
				if backendRef.Weight != nil {
					weight = int(*backendRef.Weight)
				}
				if backendRef.Kind != nil {
					kind = string(*backendRef.Kind)
				}
				namespace := namespace
				if backendRef.Namespace != nil {
					namespace = string(*backendRef.Namespace)
				}
				var port int
				if backendRef.Port != nil {
					port = int(*backendRef.Port)
				}
				backendRefsRules = append(backendRefsRules, &apimodel.BackendRefsRule{
					Name:      string(backendRef.Name),
					Weight:    weight,
					Kind:      kind,
					Namespace: namespace,
					Port:      port,
				})
			}
		}
		rules = append(rules, &apimodel.Rules{
			MatchesRules:     matchesRules,
			BackendRefsRules: backendRefsRules,
			FiltersRules:     filtersRules,
		})
	}
	var id string
	if route.Labels != nil {
		id = route.Labels["app_id"]
	}
	req.Hosts = hosts
	req.AppID = id
	req.GatewayName = gatewayName
	req.GatewayNamespace = gatewayNamespace
	req.Name = name
	req.SectionName = sectionName
	req.Namespace = namespace
	req.Rules = rules
	return &req, nil
}

// UpdateGatewayHTTPRoute update gateway http route
func (g *GatewayAction) UpdateGatewayHTTPRoute(req *apimodel.GatewayHTTPRouteStruct) (*model.K8sResource, error) {
	rules := handleGatewayRules(req)
	gatewayNamespace := v1beta1.Namespace(req.GatewayNamespace)
	var hosts []v1beta1.Hostname
	for _, host := range req.Hosts {
		hosts = append(hosts, v1beta1.Hostname(host))
	}
	httpRoute, err := g.gatewayClient.HTTPRoutes(req.Namespace).Get(context.Background(), req.Name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("update gateway http route get failure: %v", err)
		return nil, err
	}
	var sectionName *v1beta1.SectionName
	if req.SectionName != "" {
		sn := v1beta1.SectionName(req.SectionName)
		sectionName = &sn
	}
	httpRoute.Spec.Hostnames = hosts
	httpRoute.Spec.ParentRefs = []v1beta1.ParentReference{{
		Name:        v1beta1.ObjectName(req.GatewayName),
		Namespace:   &gatewayNamespace,
		SectionName: sectionName,
	}}
	httpRoute.Spec.Rules = rules
	newHTTPRoute, err := g.gatewayClient.HTTPRoutes(req.Namespace).Update(context.Background(), httpRoute, metav1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("update gateway http route update failure: %v", err)
		return nil, err
	}
	newHTTPRoute.Kind = apimodel.HTTPRoute
	newHTTPRoute.APIVersion = controller.APIVersionHTTPRoute
	httpRouteYaml, err := ObjectToJSONORYaml("yaml", &newHTTPRoute)
	if err != nil {
		logrus.Errorf("update gateway http route object to yaml failure: %v", err)
		return nil, err
	}
	res, err := db.GetManager().K8sResourceDao().GetK8sResourceByName(req.AppID, req.Name, apimodel.HTTPRoute)
	res.ErrorOverview = "更新成功"
	res.Content = httpRouteYaml
	res.State = apimodel.UpdateSuccess
	err = db.GetManager().K8sResourceDao().UpdateModel(&res)
	if err != nil {
		logrus.Errorf("database operation gateway http route update k8s resource failure: %v", err)
		return nil, err
	}
	return &res, nil
}

// DeleteGatewayHTTPRoute delete gateway http route
func (g *GatewayAction) DeleteGatewayHTTPRoute(name, namespace, appID string) error {
	err := g.gatewayClient.HTTPRoutes(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		logrus.Errorf("delete gateway http route failure: %v", err)
		return err
	}
	err = db.GetManager().K8sResourceDao().DeleteK8sResource(appID, name, apimodel.HTTPRoute)
	if err != nil {
		logrus.Errorf("database operation gateway http route delete k8s resource failure: %v", err)
		return err
	}
	return nil
}

// AddHTTPRule adds http rule to db if it doesn't exists.
func (g *GatewayAction) AddHTTPRule(req *apimodel.AddHTTPRuleStruct) error {
	return db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		if err := g.CreateHTTPRule(tx, req); err != nil {
			return err
		}

		// Effective immediately
		err := g.SendTaskDeprecated(map[string]interface{}{
			"service_id": req.ServiceID,
			"action":     "add-http-rule",
			"limit":      map[string]string{"domain": req.Domain},
		})
		if err != nil {
			return fmt.Errorf("send http rule task: %v", err)
		}

		return nil
	})
}

// CreateHTTPRule Create http rules through transactions
func (g *GatewayAction) CreateHTTPRule(tx *gorm.DB, req *apimodel.AddHTTPRuleStruct) error {
	httpRule := &model.HTTPRule{
		UUID:          req.HTTPRuleID,
		ServiceID:     req.ServiceID,
		ContainerPort: req.ContainerPort,
		Domain:        req.Domain,
		Path: func() string {
			if !strings.HasPrefix(req.Path, "/") {
				return "/" + req.Path
			}
			return req.Path
		}(),
		Header:        req.Header,
		Cookie:        req.Cookie,
		Weight:        req.Weight,
		IP:            req.IP,
		CertificateID: req.CertificateID,
		PathRewrite:   req.PathRewrite,
	}
	if err := db.GetManager().HTTPRuleDaoTransactions(tx).AddModel(httpRule); err != nil {
		return fmt.Errorf("create http rule: %v", err)
	}

	if len(req.Rewrites) > 0 {
		for _, rewrite := range req.Rewrites {
			r := &model.HTTPRuleRewrite{
				UUID:        util.NewUUID(),
				HTTPRuleID:  httpRule.UUID,
				Regex:       rewrite.Regex,
				Replacement: rewrite.Replacement,
				Flag:        rewrite.Flag,
			}
			if err := db.GetManager().HTTPRuleRewriteDaoTransactions(tx).AddModel(r); err != nil {
				return fmt.Errorf("create http rule rewrite: %v", err)
			}
		}
	}

	if strings.Replace(req.CertificateID, " ", "", -1) != "" {
		cert := &model.Certificate{
			UUID:            req.CertificateID,
			CertificateName: fmt.Sprintf("cert-%s", util.NewUUID()[0:8]),
			Certificate:     req.Certificate,
			PrivateKey:      req.PrivateKey,
		}
		if err := db.GetManager().CertificateDaoTransactions(tx).AddOrUpdate(cert); err != nil {
			return fmt.Errorf("create or update http rule: %v", err)
		}
	}

	for _, ruleExtension := range req.RuleExtensions {
		re := &model.RuleExtension{
			UUID:   util.NewUUID(),
			RuleID: httpRule.UUID,
			Key:    ruleExtension.Key,
			Value:  ruleExtension.Value,
		}
		if err := db.GetManager().RuleExtensionDaoTransactions(tx).AddModel(re); err != nil {
			return fmt.Errorf("create rule extensions: %v", err)
		}
	}

	return nil
}

// UpdateHTTPRule updates http rule
func (g *GatewayAction) UpdateHTTPRule(req *apimodel.UpdateHTTPRuleStruct) error {
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	rule, err := g.dbmanager.HTTPRuleDaoTransactions(tx).GetHTTPRuleByID(req.HTTPRuleID)
	if err != nil {
		tx.Rollback()
		return err
	}
	if rule == nil || rule.UUID == "" { // rule won't be nil
		tx.Rollback()
		return fmt.Errorf("HTTPRule dosen't exist based on uuid(%s)", req.HTTPRuleID)
	}

	// delete old http rule rewrites
	if err := db.GetManager().HTTPRuleRewriteDaoTransactions(tx).DeleteByHTTPRuleID(rule.UUID); err != nil {
		tx.Rollback()
		return err
	}
	if len(req.Rewrites) > 0 {
		// add new http rule rewrites
		for _, rewrite := range req.Rewrites {
			r := &model.HTTPRuleRewrite{
				UUID:        util.NewUUID(),
				HTTPRuleID:  rule.UUID,
				Regex:       rewrite.Regex,
				Replacement: rewrite.Replacement,
				Flag:        rewrite.Flag,
			}
			if err := db.GetManager().HTTPRuleRewriteDaoTransactions(tx).AddModel(r); err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	if strings.Replace(req.CertificateID, " ", "", -1) != "" {
		// add new certificate
		cert := &model.Certificate{
			UUID:        req.CertificateID,
			Certificate: req.Certificate,
			PrivateKey:  req.PrivateKey,
		}
		if err := g.dbmanager.CertificateDaoTransactions(tx).AddOrUpdate(cert); err != nil {
			tx.Rollback()
			return err
		}
		rule.CertificateID = req.CertificateID
	} else {
		rule.CertificateID = ""
	}
	if len(req.RuleExtensions) > 0 {
		// delete old RuleExtensions
		if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(rule.UUID); err != nil {
			tx.Rollback()
			return err
		}
		// add new rule extensions
		for _, ruleExtension := range req.RuleExtensions {
			re := &model.RuleExtension{
				UUID:   util.NewUUID(),
				RuleID: rule.UUID,
				Key:    ruleExtension.Key,
				Value:  ruleExtension.Value,
			}
			if err := db.GetManager().RuleExtensionDaoTransactions(tx).AddModel(re); err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	// update http rule
	if req.ServiceID != "" {
		rule.ServiceID = req.ServiceID
	}
	if req.ContainerPort != 0 {
		rule.ContainerPort = req.ContainerPort
	}
	if req.Domain != "" {
		rule.Domain = req.Domain
	}
	rule.Path = func() string {
		if !strings.HasPrefix(req.Path, "/") {
			return "/" + req.Path
		}
		return req.Path
	}()
	rule.Header = req.Header
	rule.Cookie = req.Cookie
	rule.Weight = req.Weight
	rule.PathRewrite = req.PathRewrite
	if req.IP != "" {
		rule.IP = req.IP
	}
	if err := db.GetManager().HTTPRuleDaoTransactions(tx).UpdateModel(rule); err != nil {
		tx.Rollback()
		return err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := g.SendTaskDeprecated(map[string]interface{}{
		"service_id": rule.ServiceID,
		"action":     "update-http-rule",
		"limit":      map[string]string{"domain": req.Domain},
	}); err != nil {
		logrus.Errorf("send runtime message about gateway failure %s", err.Error())
	}
	return nil
}

// DeleteHTTPRule deletes http rule, including certificate and rule extensions
func (g *GatewayAction) DeleteHTTPRule(req *apimodel.DeleteHTTPRuleStruct) error {
	// begin transaction
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	// delete http rule
	httpRule, err := g.dbmanager.HTTPRuleDaoTransactions(tx).GetHTTPRuleByID(req.HTTPRuleID)
	if err != nil {
		tx.Rollback()
		return err
	}
	svcID := httpRule.ServiceID
	if err := g.dbmanager.HTTPRuleDaoTransactions(tx).DeleteHTTPRuleByID(httpRule.UUID); err != nil {
		tx.Rollback()
		return err
	}

	// delete http rule rewrites
	if err := db.GetManager().HTTPRuleRewriteDaoTransactions(tx).DeleteByHTTPRuleID(httpRule.UUID); err != nil {
		tx.Rollback()
		return err
	}

	// delete rule extension
	if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(httpRule.UUID); err != nil {
		tx.Rollback()
		return err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		return err
	}

	if err := g.SendTaskDeprecated(map[string]interface{}{
		"service_id": svcID,
		"action":     "delete-http-rule",
		"limit":      map[string]string{"domain": httpRule.Domain},
	}); err != nil {
		logrus.Errorf("send runtime message about gateway failure %s", err.Error())
	}
	return nil
}

// DeleteHTTPRuleByServiceIDWithTransaction deletes http rule, including certificate and rule extensions
func (g *GatewayAction) DeleteHTTPRuleByServiceIDWithTransaction(sid string, tx *gorm.DB) error {
	// delete http rule
	rules, err := g.dbmanager.HTTPRuleDaoTransactions(tx).ListByServiceID(sid)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if err := db.GetManager().HTTPRuleRewriteDaoTransactions(tx).DeleteByHTTPRuleID(rule.UUID); err != nil {
			return err
		}
		if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(rule.UUID); err != nil {
			return err
		}
		if err := g.dbmanager.GwRuleConfigDaoTransactions(tx).DeleteByRuleID(rule.UUID); err != nil {
			return err
		}
		if err := g.dbmanager.HTTPRuleDaoTransactions(tx).DeleteHTTPRuleByID(rule.UUID); err != nil {
			return err
		}
	}

	return nil
}

// AddCertificate adds certificate to db if it doesn't exists
func (g *GatewayAction) AddCertificate(req *apimodel.AddHTTPRuleStruct, tx *gorm.DB) error {
	cert := &model.Certificate{
		UUID:            req.CertificateID,
		CertificateName: fmt.Sprintf("cert-%s", util.NewUUID()[0:8]),
		Certificate:     req.Certificate,
		PrivateKey:      req.PrivateKey,
	}

	return g.dbmanager.CertificateDaoTransactions(tx).AddModel(cert)
}

// UpdateCertificate updates certificate for http rule
func (g *GatewayAction) UpdateCertificate(req apimodel.AddHTTPRuleStruct, httpRule *model.HTTPRule,
	tx *gorm.DB) error {
	// delete old certificate
	cert, err := g.dbmanager.CertificateDaoTransactions(tx).GetCertificateByID(req.CertificateID)
	if err != nil {
		return err
	}
	if cert == nil {
		return fmt.Errorf("certificate doesn't exist based on certificateID(%s)", req.CertificateID)
	}

	cert.CertificateName = fmt.Sprintf("cert-%s", util.NewUUID()[0:8])
	cert.Certificate = req.Certificate
	cert.PrivateKey = req.PrivateKey
	return g.dbmanager.CertificateDaoTransactions(tx).UpdateModel(cert)
}

// AddTCPRule adds tcp rule.
func (g *GatewayAction) AddTCPRule(req *apimodel.AddTCPRuleStruct) error {
	return g.dbmanager.DB().Transaction(func(tx *gorm.DB) error {
		if err := g.CreateTCPRule(tx, req); err != nil {
			return err
		}

		err := g.SendTaskDeprecated(map[string]interface{}{
			"service_id": req.ServiceID,
			"action":     "add-tcp-rule",
			"limit":      map[string]string{"tcp-address": fmt.Sprintf("%s:%d", req.IP, req.Port)},
		})
		if err != nil {
			return fmt.Errorf("send tcp rule task: %v", err)
		}

		return nil
	})
}

// CreateTCPRule Create tcp rules through transactions
func (g *GatewayAction) CreateTCPRule(tx *gorm.DB, req *apimodel.AddTCPRuleStruct) error {
	// add tcp rule
	tcpRule := &model.TCPRule{
		UUID:          req.TCPRuleID,
		ServiceID:     req.ServiceID,
		ContainerPort: req.ContainerPort,
		IP:            req.IP,
		Port:          req.Port,
	}
	if err := g.dbmanager.TCPRuleDaoTransactions(tx).AddModel(tcpRule); err != nil {
		return err
	}
	// add rule extensions
	for _, ruleExtension := range req.RuleExtensions {
		re := &model.RuleExtension{
			UUID:   util.NewUUID(),
			RuleID: tcpRule.UUID,
			Value:  ruleExtension.Value,
		}
		if err := g.dbmanager.RuleExtensionDaoTransactions(tx).AddModel(re); err != nil {
			return err
		}
	}

	return nil
}

// UpdateTCPRule updates a tcp rule
func (g *GatewayAction) UpdateTCPRule(req *apimodel.UpdateTCPRuleStruct, minPort int) error {
	// begin transaction
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	// get old tcp rule
	tcpRule, err := g.dbmanager.TCPRuleDaoTransactions(tx).GetTCPRuleByID(req.TCPRuleID)
	if err != nil {
		tx.Rollback()
		return err
	}
	if len(req.RuleExtensions) > 0 {
		// delete old rule extensions
		if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(tcpRule.UUID); err != nil {
			logrus.Debugf("TCP rule id: %s;error delete rule extension: %v", tcpRule.UUID, err)
			tx.Rollback()
			return err
		}
		// add new rule extensions
		for _, ruleExtension := range req.RuleExtensions {
			re := &model.RuleExtension{
				UUID:   util.NewUUID(),
				RuleID: tcpRule.UUID,
				Value:  ruleExtension.Value,
			}
			if err := g.dbmanager.RuleExtensionDaoTransactions(tx).AddModel(re); err != nil {
				tx.Rollback()
				logrus.Debugf("TCP rule id: %s;error add rule extension: %v", tcpRule.UUID, err)
				return err
			}
		}
	}
	// update tcp rule
	if req.ContainerPort != 0 {
		tcpRule.ContainerPort = req.ContainerPort
	}
	if req.IP != "" {
		tcpRule.IP = req.IP
	}
	tcpRule.Port = req.Port
	if req.ServiceID != "" {
		tcpRule.ServiceID = req.ServiceID
	}
	if err := g.dbmanager.TCPRuleDaoTransactions(tx).UpdateModel(tcpRule); err != nil {
		logrus.Debugf("TCP rule id: %s;error updating tcp rule: %v", tcpRule.UUID, err)
		tx.Rollback()
		return err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		logrus.Debugf("TCP rule id: %s;error end transaction %v", tcpRule.UUID, err)
		return err
	}
	if err := g.SendTaskDeprecated(map[string]interface{}{
		"service_id": tcpRule.ServiceID,
		"action":     "update-tcp-rule",
		"limit":      map[string]string{"tcp-address": fmt.Sprintf("%s:%d", tcpRule.IP, tcpRule.Port)},
	}); err != nil {
		logrus.Errorf("send runtime message about gateway failure %s", err.Error())
	}
	return nil
}

// DeleteTCPRule deletes a tcp rule
func (g *GatewayAction) DeleteTCPRule(req *apimodel.DeleteTCPRuleStruct) error {
	// begin transaction
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	tcpRule, err := db.GetManager().TCPRuleDaoTransactions(tx).GetTCPRuleByID(req.TCPRuleID)
	if err != nil {
		tx.Rollback()
		return err
	}
	// delete rule extensions
	if err := db.GetManager().RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(tcpRule.UUID); err != nil {
		tx.Rollback()
		return err
	}
	// delete tcp rule
	if err := db.GetManager().TCPRuleDaoTransactions(tx).DeleteByID(tcpRule.UUID); err != nil {
		tx.Rollback()
		return err
	}
	// delete LBMappingPort
	err = db.GetManager().TenantServiceLBMappingPortDaoTransactions(tx).DELServiceLBMappingPortByServiceIDAndPort(
		tcpRule.ServiceID, tcpRule.Port)
	if err != nil {
		tx.Rollback()
		return err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := g.SendTaskDeprecated(map[string]interface{}{
		"service_id": tcpRule.ServiceID,
		"action":     "delete-tcp-rule",
		"limit":      map[string]string{"tcp-address": fmt.Sprintf("%s:%d", tcpRule.IP, tcpRule.Port)},
	}); err != nil {
		logrus.Errorf("send runtime message about gateway failure %s", err.Error())
	}
	return nil
}

// DeleteTCPRuleByServiceIDWithTransaction deletes a tcp rule
func (g *GatewayAction) DeleteTCPRuleByServiceIDWithTransaction(sid string, tx *gorm.DB) error {
	rules, err := db.GetManager().TCPRuleDaoTransactions(tx).GetTCPRuleByServiceID(sid)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		// delete rule extensions
		if err := db.GetManager().RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(rule.UUID); err != nil {
			return err
		}
		// delete tcp rule
		if err := db.GetManager().TCPRuleDaoTransactions(tx).DeleteByID(rule.UUID); err != nil {
			return err
		}
	}
	return nil
}

// AddRuleExtensions adds rule extensions to db if any of they doesn't exists
func (g *GatewayAction) AddRuleExtensions(ruleID string, ruleExtensions []*apimodel.RuleExtensionStruct,
	tx *gorm.DB) error {
	for _, ruleExtension := range ruleExtensions {
		re := &model.RuleExtension{
			UUID:   util.NewUUID(),
			RuleID: ruleID,
			Value:  ruleExtension.Value,
		}
		err := g.dbmanager.RuleExtensionDaoTransactions(tx).AddModel(re)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetAvailablePort returns a available port
func (g *GatewayAction) GetAvailablePort(ip string, lock bool) (int, error) {
	roles, err := g.dbmanager.TCPRuleDao().GetUsedPortsByIP(ip)
	if err != nil {
		return 0, err
	}
	var ports []int
	for _, p := range roles {
		ports = append(ports, p.Port)
	}
	//resp, err := clientv3.KV(g.etcdCli).Get(context.TODO(), "/rainbond/gateway/lockports", clientv3.WithPrefix())
	//if err != nil {
	//	logrus.Info("get lock ports failed")
	//}
	//for _, etcdValue := range resp.Kvs {
	//	port, err := strconv.Atoi(string(etcdValue.Value))
	//	if err != nil {
	//		continue
	//	}
	//	ports = append(ports, port)
	//}
	//port := selectAvailablePort(ports)
	//if port != 0 {
	//	if lock {
	//		lease := clientv3.NewLease(g.etcdCli)
	//		leaseResp, err := lease.Grant(context.Background(), 120)
	//		if err != nil {
	//			logrus.Info("set lease failed")
	//			return port, nil
	//		}
	//		lockPortKey := fmt.Sprintf("/rainbond/gateway/lockports/%d", port)
	//		_, err = g.etcdCli.Put(context.Background(), lockPortKey, fmt.Sprintf("%d", port), clientv3.WithLease(leaseResp.ID))
	//		if err != nil {
	//			logrus.Infof("set lock port key %s failed", lockPortKey)
	//			return port, nil
	//		}
	//		logrus.Infof("select gateway port %d, lock it 2 min", port)
	//	}
	//	return port, nil
	//}
	return 0, fmt.Errorf("no more lb port can be use with ip %s", ip)
}

func selectAvailablePort(used []int) int {
	maxPort, _ := strconv.Atoi(os.Getenv("MAX_LB_PORT"))
	minPort, _ := strconv.Atoi(os.Getenv("MIN_LB_PORT"))
	if minPort == 0 {
		minPort = 10000
	}
	if maxPort == 0 {
		maxPort = 65535
	}
	if len(used) == 0 {
		return minPort
	}

	sort.Ints(used)
	selectPort := used[len(used)-1] + 1
	if selectPort < minPort {
		selectPort = minPort
	}
	//顺序分配端口
	if selectPort <= maxPort {
		return selectPort
	}
	//捡漏以前端口
	selectPort = minPort
	for _, p := range used {
		if p == selectPort {
			selectPort = selectPort + 1
			continue
		}
		if p > selectPort {
			return selectPort
		}
		selectPort = selectPort + 1
	}
	if selectPort <= maxPort {
		return selectPort
	}
	return 0
}

// TCPIPPortExists returns if the port exists
func (g *GatewayAction) TCPIPPortExists(host string, port int) bool {
	roles, _ := db.GetManager().TCPRuleDao().GetUsedPortsByIP(host)
	for _, role := range roles {
		if role.Port == port {
			return true
		}
	}
	return false
}

// SendTaskDeprecated sends apply rules task
func (g *GatewayAction) SendTaskDeprecated(in map[string]interface{}) error {
	sid := in["service_id"].(string)
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(sid)
	if err != nil {
		return fmt.Errorf("unexpected error occurred while getting Service by ServiceID(%s): %v", sid, err)
	}
	body := make(map[string]interface{})
	body["deploy_version"] = service.DeployVersion
	for k, v := range in {
		body[k] = v
	}
	err = g.mqclient.SendBuilderTopic(client.TaskStruct{
		Topic:    client.WorkerTopic,
		TaskType: "apply_rule",
		TaskBody: body,
	})
	if err != nil {
		return fmt.Errorf("unexpected error occurred while sending task: %v", err)
	}
	return nil
}

// SendTask sends apply rules task
func (g *GatewayAction) SendTask(task *ComponentIngressTask) error {
	err := g.mqclient.SendBuilderTopic(client.TaskStruct{
		Topic:    client.WorkerTopic,
		TaskType: "apply_rule",
		TaskBody: task,
	})
	if err != nil {
		return errors.WithMessage(err, "send gateway task")
	}
	return nil
}

// RuleConfig -
func (g *GatewayAction) RuleConfig(req *apimodel.RuleConfigReq) error {
	var configs []*model.GwRuleConfig
	// TODO: use reflect to read the field of req, huangrh
	configs = append(configs, &model.GwRuleConfig{
		RuleID: req.RuleID,
		Key:    "proxy-connect-timeout",
		Value:  strconv.Itoa(req.Body.ProxyConnectTimeout),
	})
	configs = append(configs, &model.GwRuleConfig{
		RuleID: req.RuleID,
		Key:    "proxy-send-timeout",
		Value:  strconv.Itoa(req.Body.ProxySendTimeout),
	})
	configs = append(configs, &model.GwRuleConfig{
		RuleID: req.RuleID,
		Key:    "proxy-read-timeout",
		Value:  strconv.Itoa(req.Body.ProxyReadTimeout),
	})
	configs = append(configs, &model.GwRuleConfig{
		RuleID: req.RuleID,
		Key:    "proxy-body-size",
		Value:  strconv.Itoa(req.Body.ProxyBodySize),
	})
	configs = append(configs, &model.GwRuleConfig{
		RuleID: req.RuleID,
		Key:    "proxy-buffer-size",
		Value:  strconv.Itoa(req.Body.ProxyBufferSize) + "k",
	})
	configs = append(configs, &model.GwRuleConfig{
		RuleID: req.RuleID,
		Key:    "proxy-buffer-numbers",
		Value:  strconv.Itoa(req.Body.ProxyBufferNumbers),
	})
	configs = append(configs, &model.GwRuleConfig{
		RuleID: req.RuleID,
		Key:    "proxy-buffering",
		Value:  req.Body.ProxyBuffering,
	})
	setheaders := make(map[string]string)
	for _, item := range req.Body.SetHeaders {
		if strings.TrimSpace(item.Key) == "" {
			continue
		}
		if strings.TrimSpace(item.Value) == "" {
			item.Value = "empty"
		}
		// filter same key
		setheaders["set-header-"+item.Key] = item.Value
	}
	for k, v := range setheaders {
		configs = append(configs, &model.GwRuleConfig{
			RuleID: req.RuleID,
			Key:    k,
			Value:  v,
		})
	}

	// response headers
	responseHeaders := make(map[string]string)
	for _, item := range req.Body.ResponseHeaders {
		if strings.TrimSpace(item.Key) == "" {
			continue
		}
		if strings.TrimSpace(item.Value) == "" {
			item.Value = "empty"
		}
		// filter same key
		responseHeaders["resp-header-"+item.Key] = item.Value
	}
	for k, v := range responseHeaders {
		configs = append(configs, &model.GwRuleConfig{
			RuleID: req.RuleID,
			Key:    k,
			Value:  v,
		})
	}

	rule, err := g.dbmanager.HTTPRuleDao().GetHTTPRuleByID(req.RuleID)
	if err != nil {
		return err
	}

	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	if err := g.dbmanager.GwRuleConfigDaoTransactions(tx).DeleteByRuleID(req.RuleID); err != nil {
		tx.Rollback()
		return err
	}
	for _, cfg := range configs {
		if err := g.dbmanager.GwRuleConfigDaoTransactions(tx).AddModel(cfg); err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := g.SendTaskDeprecated(map[string]interface{}{
		"service_id": req.ServiceID,
		"action":     "update-rule-config",
		"event_id":   req.EventID,
		"limit":      map[string]string{"domain": rule.Domain},
	}); err != nil {
		logrus.Errorf("send runtime message about gateway failure %s", err.Error())
	}
	return nil
}

// UpdCertificate -
func (g *GatewayAction) UpdCertificate(req *apimodel.UpdCertificateReq) error {
	cert, err := db.GetManager().CertificateDao().GetCertificateByID(req.CertificateID)
	if err != nil {
		msg := "retrieve certificate: %v"
		return fmt.Errorf(msg, err)
	}

	if cert == nil {
		// cert do not exists in region db, create it
		cert = &model.Certificate{
			UUID:            req.CertificateID,
			CertificateName: req.CertificateName,
			Certificate:     req.Certificate,
			PrivateKey:      req.PrivateKey,
		}
		if err := db.GetManager().CertificateDao().AddModel(cert); err != nil {
			msg := "update cert error :%s"
			return fmt.Errorf(msg, err.Error())
		}
		return nil
	}

	cert.CertificateName = req.CertificateName
	cert.Certificate = req.Certificate
	cert.PrivateKey = req.PrivateKey
	if err := db.GetManager().CertificateDao().UpdateModel(cert); err != nil {
		msg := "update certificate: %v"
		return fmt.Errorf(msg, err)
	}

	// list related http rules
	rules, err := g.ListHTTPRulesByCertID(req.CertificateID)
	if err != nil {
		msg := "certificate id: %s; list http rules: %v"
		return fmt.Errorf(msg, req.CertificateID, err)
	}

	for _, rule := range rules {
		eventID := util.NewUUID()
		if err := g.SendTaskDeprecated(map[string]interface{}{
			"service_id": rule.ServiceID,
			"action":     "update-rule-config",
			"event_id":   eventID,
			"limit":      map[string]string{"domain": rule.Domain},
		}); err != nil {
			logrus.Warningf("send runtime message about gateway failure %v", err)
		}
	}

	return nil
}

// ListHTTPRulesByCertID -
func (g *GatewayAction) ListHTTPRulesByCertID(certID string) ([]*model.HTTPRule, error) {
	return db.GetManager().HTTPRuleDao().ListByCertID(certID)
}

// IPAndAvailablePort ip and advice available port
type IPAndAvailablePort struct {
	IP            string `json:"ip"`
	AvailablePort int    `json:"available_port"`
}

// GetGatewayIPs get all gateway node ips
func (g *GatewayAction) GetGatewayIPs() []IPAndAvailablePort {
	defaultAvailablePort, _ := g.GetAvailablePort("0.0.0.0", false)
	defaultIps := []IPAndAvailablePort{{
		IP:            "0.0.0.0",
		AvailablePort: defaultAvailablePort,
	}}
	res, err := db.GetManager().KeyValueDao().WithPrefix("/rainbond/gateway/ips")
	if err != nil || len(res) == 0 {
		return defaultIps
	}

	var gatewayIps = make([]string, 0)
	for _, v := range res {
		gatewayIps = append(gatewayIps, v.V)
	}
	sort.Strings(gatewayIps)
	for _, v := range gatewayIps {
		availablePort, _ := g.GetAvailablePort(v, false)
		defaultIps = append(defaultIps, IPAndAvailablePort{
			IP:            v,
			AvailablePort: availablePort,
		})
	}
	return defaultIps
}

// DeleteIngressRulesByComponentPort deletes ingress rules, including http rules and tcp rules, based on the given componentID and port.
func (g *GatewayAction) DeleteIngressRulesByComponentPort(tx *gorm.DB, componentID string, port int) error {
	httpRuleIDs, err := g.listHTTPRuleIDs(componentID, port)
	if err != nil {
		return err
	}

	// delete rule configs
	if err := db.GetManager().GwRuleConfigDaoTransactions(tx).DeleteByRuleIDs(httpRuleIDs); err != nil {
		return err
	}

	// delete rule extentions
	if err := db.GetManager().RuleExtensionDaoTransactions(tx).DeleteByRuleIDs(httpRuleIDs); err != nil {
		return err
	}

	// delete http rules
	if err := db.GetManager().HTTPRuleDaoTransactions(tx).DeleteByComponentPort(componentID, port); err != nil {
		if !errors.Is(err, bcode.ErrIngressHTTPRuleNotFound) {
			return err
		}
	}

	// delete tcp rules
	if err := db.GetManager().TCPRuleDaoTransactions(tx).DeleteByComponentPort(componentID, port); err != nil {
		if !errors.Is(err, bcode.ErrIngressTCPRuleNotFound) {
			return err
		}
	}

	return nil
}

func (g *GatewayAction) listHTTPRuleIDs(componentID string, port int) ([]string, error) {
	httpRules, err := db.GetManager().HTTPRuleDao().ListByComponentPort(componentID, port)
	if err != nil {
		return nil, err
	}

	var ruleIDs []string
	for _, rule := range httpRules {
		ruleIDs = append(ruleIDs, rule.UUID)
	}
	return ruleIDs, nil
}

// SyncHTTPRules -
func (g *GatewayAction) SyncHTTPRules(tx *gorm.DB, components []*apimodel.Component) error {
	var (
		componentIDs     []string
		httpRules        []*model.HTTPRule
		ruleExtensions   []*model.RuleExtension
		httpRuleRewrites []*model.HTTPRuleRewrite
	)
	for _, component := range components {
		if len(component.HTTPRules) == 0 {
			continue
		}
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		for _, httpRule := range component.HTTPRules {
			httpRules = append(httpRules, httpRule.DbModel(component.ComponentBase.ComponentID))

			for _, rewrite := range httpRule.Rewrites {
				httpRuleRewrites = append(httpRuleRewrites, &model.HTTPRuleRewrite{
					UUID:        util.NewUUID(),
					HTTPRuleID:  httpRule.HTTPRuleID,
					Regex:       rewrite.Regex,
					Replacement: rewrite.Replacement,
					Flag:        rewrite.Flag,
				})
			}

			for _, ext := range httpRule.RuleExtensions {
				ruleExtensions = append(ruleExtensions, &model.RuleExtension{
					UUID:   util.NewUUID(),
					RuleID: httpRule.HTTPRuleID,
					Key:    ext.Key,
					Value:  ext.Value,
				})
			}
		}
	}

	if err := g.syncHTTPRuleRewrites(tx, httpRules, httpRuleRewrites); err != nil {
		return err
	}

	if err := g.syncRuleExtensions(tx, httpRules, ruleExtensions); err != nil {
		return err
	}

	if err := db.GetManager().HTTPRuleDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	return db.GetManager().HTTPRuleDaoTransactions(tx).CreateOrUpdateHTTPRuleInBatch(httpRules)
}

func (g *GatewayAction) syncHTTPRuleRewrites(tx *gorm.DB, httpRules []*model.HTTPRule, rewrites []*model.HTTPRuleRewrite) error {
	var ruleIDs []string
	for _, hr := range httpRules {
		ruleIDs = append(ruleIDs, hr.UUID)
	}

	if err := db.GetManager().HTTPRuleRewriteDaoTransactions(tx).DeleteByHTTPRuleIDs(ruleIDs); err != nil {
		return err
	}
	return db.GetManager().HTTPRuleRewriteDaoTransactions(tx).CreateOrUpdateHTTPRuleRewriteInBatch(rewrites)
}

func (g *GatewayAction) syncRuleExtensions(tx *gorm.DB, httpRules []*model.HTTPRule, exts []*model.RuleExtension) error {
	var ruleIDs []string
	for _, hr := range httpRules {
		ruleIDs = append(ruleIDs, hr.UUID)
	}

	if err := db.GetManager().RuleExtensionDaoTransactions(tx).DeleteByRuleIDs(ruleIDs); err != nil {
		return err
	}
	return db.GetManager().RuleExtensionDaoTransactions(tx).CreateOrUpdateRuleExtensionsInBatch(exts)
}

// SyncTCPRules -
func (g *GatewayAction) SyncTCPRules(tx *gorm.DB, components []*apimodel.Component) error {
	var (
		componentIDs []string
		tcpRules     []*model.TCPRule
	)
	for _, component := range components {
		if len(component.TCPRules) == 0 {
			continue
		}
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		for _, tcpRule := range component.TCPRules {
			tcpRules = append(tcpRules, tcpRule.DbModel(component.ComponentBase.ComponentID))
		}
	}
	if err := db.GetManager().TCPRuleDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	return db.GetManager().TCPRuleDaoTransactions(tx).CreateOrUpdateTCPRuleInBatch(tcpRules)
}

// SyncRuleConfigs -
func (g *GatewayAction) SyncRuleConfigs(tx *gorm.DB, components []*apimodel.Component) error {
	var configs []*model.GwRuleConfig
	var componentIDs []string
	for _, component := range components {
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		if len(component.HTTPRuleConfigs) == 0 {
			continue
		}

		for _, httpRuleConfig := range component.HTTPRuleConfigs {
			configs = append(configs, httpRuleConfig.DbModel()...)
		}
	}

	// http rule ids
	rules, err := db.GetManager().HTTPRuleDao().ListByComponentIDs(componentIDs)
	if err != nil {
		return err
	}
	var ruleIDs []string
	for _, rule := range rules {
		ruleIDs = append(ruleIDs, rule.UUID)
	}

	if err := db.GetManager().GwRuleConfigDaoTransactions(tx).DeleteByRuleIDs(ruleIDs); err != nil {
		return err
	}
	return db.GetManager().GwRuleConfigDaoTransactions(tx).CreateOrUpdateGwRuleConfigsInBatch(configs)
}
