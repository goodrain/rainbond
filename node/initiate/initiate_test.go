package initiate

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	rainbondv1alpha1 "github.com/GLYASAI/rainbond-operator/pkg/apis/rainbond/v1alpha1"
	"github.com/GLYASAI/rainbond-operator/pkg/generated/clientset/versioned/fake"
)

func TestInitiateManager_initiateFstab(t *testing.T) {
	ns := "rbd-system"
	name := "rainbondcluster"

	scheme := runtime.NewScheme()
	_ = rainbondv1alpha1.AddToScheme(scheme)
	cluster := &rainbondv1alpha1.RainbondCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: rainbondv1alpha1.RainbondClusterSpec{
			FstabLines: []rainbondv1alpha1.FstabLine{
				{
					FileSystem: "host.myserver.com:/home",
					MountPoint: "/mnt/home",
					Type:       "nfs",
					Options:    "rw,hard,intr,rsize=8192,wsize=8192,timeo=14",
					Dump:       0,
					Pass:       0,
				},
			},
		},
	}
	_ = fake.AddToScheme(scheme)
	clientset := fake.NewSimpleClientset(cluster)

	initiateManager := New(clientset, ns, name)
	if err := initiateManager.initiateFstab("./testdata/fstab"); err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
}
