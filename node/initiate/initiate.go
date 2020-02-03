package initiate

import (
	"fmt"

	rainbondv1alpha1 "github.com/GLYASAI/rainbond-operator/pkg/apis/rainbond/v1alpha1"
	rainbondv1alpha1clienset "github.com/GLYASAI/rainbond-operator/pkg/generated/clientset/versioned"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InitiateManager is responsible for initialization
type InitiateManager struct {
	rainbondClient rainbondv1alpha1clienset.Interface
	ClusterNS      string
	ClusterName    string
}

// New creates a new InitiateManager.
func New(rainbondClient rainbondv1alpha1clienset.Interface, ns, name string) *InitiateManager {
	return &InitiateManager{
		rainbondClient: rainbondClient,
		ClusterNS:      ns,
		ClusterName:    name,
	}
}

func (i *InitiateManager) Start() error {
	return i.initiateFstab("/etc/fstab")
}

func (i *InitiateManager) Stop() {
	return
}

func (i *InitiateManager) initiateFstab(path string) error {
	cluster, err := i.getRainbondClusterIfExist()
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	fstab, err := NewFstab(path)
	if err != nil {
		return err
	}
	for _, line := range cluster.Spec.FstabLines {
		raw := fmt.Sprintf("%s %s %s %s %d %d", line.FileSystem, line.MountPoint, line.Type, line.Options, line.Dump, line.Pass)
		fstab.AddIfNotExist(raw)
	}
	return fstab.Flush()
}

func (i *InitiateManager) getRainbondClusterIfExist() (*rainbondv1alpha1.RainbondCluster, error) {
	cluster, err := i.rainbondClient.RainbondV1alpha1().RainbondClusters(i.ClusterNS).Get(i.ClusterName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cluster, nil
}
