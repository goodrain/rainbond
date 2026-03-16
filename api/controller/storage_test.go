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

package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func setupStorageTestEnv() {
	// Create fake clientset
	clientset := fake.NewSimpleClientset()
	k8sComp := k8s.Default()
	k8sComp.TestClientset = clientset
}

func TestGetStorageOverview(t *testing.T) {
	setupStorageTestEnv()

	// Create test data
	clientset := k8s.Default().TestClientset

	// Create test StorageClass
	reclaimPolicy := corev1.PersistentVolumeReclaimDelete
	bindingMode := storagev1.VolumeBindingImmediate
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sc",
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": "true",
			},
		},
		Provisioner:       "test-provisioner",
		ReclaimPolicy:     &reclaimPolicy,
		VolumeBindingMode: &bindingMode,
	}
	_, err := clientset.StorageV1().StorageClasses().Create(nil, sc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test StorageClass: %v", err)
	}

	// Create test PersistentVolumes
	pv1 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv1"},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:         corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
			StorageClassName: "test-sc",
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeAvailable},
	}
	pv2 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv2"},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:         corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("20Gi")},
			StorageClassName: "test-sc",
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}
	_, err = clientset.CoreV1().PersistentVolumes().Create(nil, pv1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test PV1: %v", err)
	}
	_, err = clientset.CoreV1().PersistentVolumes().Create(nil, pv2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test PV2: %v", err)
	}

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/storage/overview", nil)
	w := httptest.NewRecorder()

	// Call controller
	GetStorageOverview(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Data handler.StorageOverview `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Data.TotalPVs != 2 {
		t.Errorf("expected 2 total PVs, got %d", response.Data.TotalPVs)
	}
	if response.Data.AvailablePVs != 1 {
		t.Errorf("expected 1 available PV, got %d", response.Data.AvailablePVs)
	}
	if response.Data.BoundPVs != 1 {
		t.Errorf("expected 1 bound PV, got %d", response.Data.BoundPVs)
	}
	if len(response.Data.StorageClasses) != 1 {
		t.Errorf("expected 1 storage class, got %d", len(response.Data.StorageClasses))
	}
}

func TestListStorageClasses(t *testing.T) {
	setupStorageTestEnv()

	// Create test data
	clientset := k8s.Default().TestClientset

	// Create test StorageClasses
	reclaimPolicy := corev1.PersistentVolumeReclaimDelete
	bindingMode := storagev1.VolumeBindingImmediate
	sc1 := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sc1",
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": "true",
			},
		},
		Provisioner:       "provisioner1",
		ReclaimPolicy:     &reclaimPolicy,
		VolumeBindingMode: &bindingMode,
	}
	sc2 := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "sc2"},
		Provisioner:       "provisioner2",
		ReclaimPolicy:     &reclaimPolicy,
		VolumeBindingMode: &bindingMode,
	}
	_, err := clientset.StorageV1().StorageClasses().Create(nil, sc1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test SC1: %v", err)
	}
	_, err = clientset.StorageV1().StorageClasses().Create(nil, sc2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test SC2: %v", err)
	}

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/storage/storageclasses", nil)
	w := httptest.NewRecorder()

	// Call controller
	ListStorageClasses(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Data []handler.StorageClassInfo `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("expected 2 storage classes, got %d", len(response.Data))
	}
}

func TestListPersistentVolumes(t *testing.T) {
	setupStorageTestEnv()

	// Create test data
	clientset := k8s.Default().TestClientset

	// Create test PersistentVolumes
	pv1 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv1"},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:         corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
			StorageClassName: "test-sc",
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeAvailable},
	}
	pv2 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv2"},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:         corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("20Gi")},
			StorageClassName: "test-sc",
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}
	_, err := clientset.CoreV1().PersistentVolumes().Create(nil, pv1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test PV1: %v", err)
	}
	_, err = clientset.CoreV1().PersistentVolumes().Create(nil, pv2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test PV2: %v", err)
	}

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/storage/pvs", nil)
	w := httptest.NewRecorder()

	// Call controller
	ListPersistentVolumes(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Data []corev1.PersistentVolume `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("expected 2 PVs, got %d", len(response.Data))
	}
}
