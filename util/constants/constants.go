package constants

const (
	// Rainbond -
	Rainbond = "rainbond"
	// DefImageRepository default private image repository
	DefImageRepository = "goodrain.me"
	// GrdataLogPath -
	GrdataLogPath = "/grdata/logs"
	// ImagePullSecretKey the key of environment IMAGE_PULL_SECRET
	ImagePullSecretKey = "IMAGE_PULL_SECRET"
	// DefOnlineImageRepository default private image repository
	DefOnlineImageRepository = "registry.cn-hangzhou.aliyuncs.com/goodrain"
)

// Kubernetes recommended Labels
// Refer to: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels
const (
	ResourceManagedByLabel = "app.kubernetes.io/managed-by"
	ResourceInstanceLabel  = "app.kubernetes.io/instance"
)
