package adaptor

import (
	"context"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
)

// AppGoveranceModeHandler Application governance mode processing interface
type AppGoveranceModeHandler interface {
	IsInstalledControlPlane() bool
	GetInjectLabels() map[string]string
}

// NewAppGoveranceModeHandler -
func NewAppGoveranceModeHandler(governanceMode string, kubeClient clientset.Interface) (AppGoveranceModeHandler, error) {
	switch governanceMode {
	case model.GovernanceModeIstioServiceMesh:
		return NewIstioGoveranceMode(kubeClient), nil
	case model.GovernanceModeBuildInServiceMesh:
		return NewBuildInServiceMeshMode(), nil
	case model.GovernanceModeKubernetesNativeService:
		return NewKubernetesNativeMode(), nil
	default:
		return nil, bcode.ErrInvalidGovernanceMode
	}
}

// IsGovernanceModeValid checks if the governanceMode is valid.
func IsGovernanceModeValid(governanceMode string, dynamicClient dynamic.Interface) bool {
	switch governanceMode {
	case model.GovernanceModeBuildInServiceMesh:
		return true
	case model.GovernanceModeKubernetesNativeService:
		return true
	case model.GovernanceModeIstioServiceMesh:
		return true
	default:
		found := findGovernanceMode(governanceMode, dynamicClient)
		logrus.Infof("find governance mode %s, found: %v", governanceMode, found)
		if found {
			return true
		}
	}
	return false
}

func findGovernanceMode(governanceMode string, dynamicClient dynamic.Interface) (found bool) {
	logrus.Infof("find governance mode %s", governanceMode)
	if dynamicClient == nil {
		return false
	}
	res := schema.GroupVersionResource{
		Group:    "rainbond.io",
		Version:  "v1alpha1",
		Resource: "servicemeshclasses",
	}
	serviceMeshClasses, err := dynamicClient.Resource(res).Get(context.Background(), governanceMode, metav1.GetOptions{})
	logrus.Infof("find governance mode %s, list: %v, err: %v", governanceMode, serviceMeshClasses, err)
	if err != nil {
		logrus.Errorf("list servicemeshclasses failure %s", err.Error())
		return false
	}
	return true
}

// NeedCreateCR need create cr
//func NeedCreateCR(governanceMode string, dynamicClient dynamic.Interface) bool {
//	switch governanceMode {
//	case model.GovernanceModeBuildInServiceMesh:
//		return false
//	case model.GovernanceModeKubernetesNativeService:
//		return false
//	case model.GovernanceModeIstioServiceMesh:
//		return false
//	default:
//		found := findGovernanceMode(governanceMode, dynamicClient)
//		logrus.Infof("find governance mode %s, found: %v", governanceMode, found)
//		if found {
//			return true
//		}
//	}
//	return false
//}
