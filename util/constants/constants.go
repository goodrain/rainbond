package constants

const (
	// Namespace -
	Namespace = "rbd-system"
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

	// NotFound
	NotFound = "not found"

	// TenantQuotaCPULack
	TenantQuotaCPULack = "tenant_quota_cpu_lack"

	// enantQuotaMemoryLack
	TenantQuotaMemoryLack = "tenant_quota_memory_lack"

	// TenantLackOfMemory
	TenantLackOfMemory = "tenant_lack_of_memory"

	// TenantLackOfCPU
	TenantLackOfCPU = "tenant_lack_of_cpu"

	// TenantLackOfStorage
	TenantLackOfStorage = "tenant_lack_of_storage"

	// ClusterLackOfMemory
	ClusterLackOfMemory = "cluster_lack_of_memory"
)

// Kubernetes recommended Labels
// Refer to: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels
const (
	ResourceManagedByLabel = "app.kubernetes.io/managed-by"
	ResourceInstanceLabel  = "app.kubernetes.io/instance"
	ResourceAppLabel       = "app"
)
