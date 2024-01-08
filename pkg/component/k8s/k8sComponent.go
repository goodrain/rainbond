package k8s

import (
	"context"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	rainbondscheme "github.com/goodrain/rainbond/pkg/generated/clientset/versioned/scheme"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"kubevirt.io/client-go/kubecli"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1beta1"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1beta1"
)

// k8sComponent -
type K8sComponent struct {
	RestConfig    *rest.Config
	Clientset     *kubernetes.Clientset
	GatewayClient *v1beta1.GatewayV1beta1Client
	DynamicClient *dynamic.DynamicClient

	RainbondClient *versioned.Clientset
	K8sClient      k8sclient.Client
	KubevirtCli    kubecli.KubevirtClient

	Mapper meta.RESTMapper
}

var defaultK8sComponent *K8sComponent

func K8sClient() *K8sComponent {
	defaultK8sComponent = &K8sComponent{}
	return defaultK8sComponent
}

func (k *K8sComponent) Start(ctx context.Context, cfg *configs.Config) error {
	logrus.Infof("init k8s client...")
	config, err := k8sutil.NewRestConfig(cfg.APIConfig.KubeConfigPath)
	k.RestConfig = config
	if err != nil {
		logrus.Errorf("create k8s config failure: %v", err)
		return err
	}
	k.Clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Errorf("create k8s client failure: %v", err)
		return err
	}
	k.GatewayClient, err = gateway.NewForConfig(config)
	if err != nil {
		logrus.Errorf("create gateway client failure: %v", err)
		return err
	}
	k.DynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		logrus.Errorf("create dynamic client failure: %v", err)
		return err
	}

	k.RainbondClient = versioned.NewForConfigOrDie(config)

	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)
	rainbondscheme.AddToScheme(scheme)
	k.K8sClient, err = k8sclient.New(config, k8sclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		logrus.Errorf("create k8s client failure: %v", err)
		return err
	}

	k.KubevirtCli, err = kubecli.GetKubevirtClientFromRESTConfig(config)
	if err != nil {
		logrus.Errorf("create kubevirt cli failure: %v", err)
		return err
	}

	gr, err := restmapper.GetAPIGroupResources(k.Clientset)
	if err != nil {
		return err
	}
	k.Mapper = restmapper.NewDiscoveryRESTMapper(gr)
	logrus.Infof("init k8s client success")
	return nil
}

func (k *K8sComponent) CloseHandle() {
}

func Default() *K8sComponent {
	return defaultK8sComponent
}
