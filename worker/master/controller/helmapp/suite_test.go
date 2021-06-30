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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goodrain/rainbond/util"
	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/goodrain/rainbond/pkg/generated/informers/externalversions"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var ctx = context.Background()
var kubeClient clientset.Interface
var rainbondClient versioned.Interface
var testEnv *envtest.Environment
var stopCh = make(chan struct{})

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"HelmApp Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	projectHome := os.Getenv("PROJECT_HOME")
	kubeconfig := os.Getenv("KUBE_CONFIG")

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join(projectHome, "config", "crd")},
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    util.Bool(true),
	}

	_, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())

	restConfig, err := k8sutil.NewRestConfig(kubeconfig)
	Expect(err).NotTo(HaveOccurred())

	rainbondClient = versioned.NewForConfigOrDie(restConfig)
	kubeClient = clientset.NewForConfigOrDie(restConfig)
	rainbondInformer := externalversions.NewSharedInformerFactoryWithOptions(rainbondClient, 10*time.Second,
		externalversions.WithNamespace(corev1.NamespaceAll))

	ctrl := NewController(ctx, stopCh, kubeClient, rainbondClient, rainbondInformer.Rainbond().V1alpha1().HelmApps().Informer(), rainbondInformer.Rainbond().V1alpha1().HelmApps().Lister(), "/tmp/helm/repo/repositories.yaml", "/tmp/helm/cache", "/tmp/helm/chart")
	go ctrl.Start()

	// create namespace

}, 60)

var _ = AfterSuite(func() {
	By("tearing down the helmCmd app controller")
	close(stopCh)

	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
