/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helmapp

import (
	"context"
	"github.com/goodrain/rainbond/util"
	clientset "k8s.io/client-go/kubernetes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
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

	ctrl := NewController(ctx, stopCh, kubeClient, rainbondClient, 5*time.Second, "/tmp/helm/repo/repositories.yaml", "/tmp/helm/cache")
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
