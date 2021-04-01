// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//CreateKubeService create kube service
func CreateKubeService(client kubernetes.Interface, namespace string, services ...*corev1.Service) error {
	var retryService []*corev1.Service
	for i := range services {
		createService := services[i]
		if _, err := client.CoreV1().Services(namespace).Create(context.Background(), createService, metav1.CreateOptions{}); err != nil {
			// Ignore if the Service is invalid with this error message:
			// 	Service "kube-dns" is invalid: spec.clusterIP: Invalid value: "10.96.0.10": provided IP is already allocated
			if !errors.IsAlreadyExists(err) && !errors.IsInvalid(err) {
				retryService = append(retryService, createService)
				continue
			}
			if _, err := client.CoreV1().Services(namespace).Update(context.Background(), createService, metav1.UpdateOptions{}); err != nil {
				retryService = append(retryService, createService)
				continue
			}
		}
	}
	//second attempt
	for _, service := range retryService {
		_, err := client.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				continue
			}
			return err
		}
	}
	return nil
}
