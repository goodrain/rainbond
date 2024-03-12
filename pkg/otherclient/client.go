package otherclient

import apisixversioned "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned"

// APISixClient - worker 想要使用 APISixClient 但是不好传递此参数，后续优化worker启动的逻辑
var apisixclient *apisixversioned.Clientset

// GetAPISixClient -
func GetAPISixClient() *apisixversioned.Clientset {
	return apisixclient
}

// SetAPISixClient -
func SetAPISixClient(c *apisixversioned.Clientset) {
	apisixclient = c
}
