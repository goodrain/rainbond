package main

import "C"
import (
	"os"
	"time"

	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/goodrain/rainbond/worker/controllers/helmapp"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.DebugLevel)
}

func main() {
	restcfg, err := k8sutil.NewRestConfig("/Users/abewang/.kube/config")
	if err != nil {
		logrus.Fatalf("create kube rest config error: %s", err.Error())
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	//clientset := versioned.NewForConfigOrDie(restcfg)

	//helmApp := &rainbondv1alpha1.HelmApp{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name:      "foo",
	//		Namespace: "rbd-system",
	//	},
	//	Spec: rainbondv1alpha1.HelmAppSpec{
	//		PreStatus: "",
	//		Version:   "1.3.0",
	//		Revision:  Int32(0),
	//		Values:    "",
	//		AppStore: &rainbondv1alpha1.HelmAppStore{
	//			Version: "1111111",
	//			Name:    "rainbond",
	//			URL:     "https://openchart.goodrain.com/goodrain/rainbond",
	//		},
	//	},
	//}
	//if _, err := clientset.RainbondV1alpha1().HelmApps("rbd-system").Create(context.Background(),
	//	helmApp, metav1.CreateOptions{}); err != nil {
	//	if !k8sErrors.IsAlreadyExists(err) {
	//		logrus.Fatal(err)
	//	}
	//}

	ctrl := helmapp.NewController(stopCh, restcfg, 5*time.Second, "/tmp/helm/repo/repositories.yaml", "/tmp/helm/cache")
	if err = ctrl.Start(); err != nil {
		logrus.Fatalf("start controller: %v", err)
	}

	select {}
}

// Int32 returns a pointer to the int32 value passed in.
func Int32(v int32) *int32 {
	return &v
}
