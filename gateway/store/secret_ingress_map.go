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

package store

import (
	"fmt"

	"github.com/goodrain/rainbond/util/ingress-nginx/k8s"
	betav1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
)

type secretIngressMap struct {
	v map[string][]string
}

func (m *secretIngressMap) update(ingress interface{}) {
	nwkIngress, ok := ingress.(*networkingv1.Ingress)
	if ok {
		ingKey := k8s.MetaNamespaceKey(nwkIngress)
		for _, tls := range nwkIngress.Spec.TLS {
			secretKey := fmt.Sprintf("%s/%s", nwkIngress.Namespace, tls.SecretName)
			m.v[ingKey] = append(m.v[ingKey], secretKey)
		}
	} else {
		betaIngress, ok := ingress.(*betav1.Ingress)
		if ok {
			ingKey := k8s.MetaNamespaceKey(betaIngress)
			for _, tls := range betaIngress.Spec.TLS {
				secretKey := fmt.Sprintf("%s/%s", betaIngress.Namespace, tls.SecretName)
				m.v[ingKey] = append(m.v[ingKey], secretKey)
			}
		}
	}

}

func (m *secretIngressMap) getSecretKeys(ingKey string) []string {
	return m.v[ingKey]
}
