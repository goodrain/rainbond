package model

const (
	//Deployment -
	Deployment = "Deployment"
	//Job -
	Job = "Job"
	//CronJob -
	CronJob = "CronJob"
	//StateFulSet -
	StateFulSet = "StatefulSet"
	//Service -
	Service = "Service"
	//PVC -
	PVC = "PersistentVolumeClaim"
	//Ingress -
	Ingress = "Ingress"
	//NetworkPolicy -
	NetworkPolicy = "NetworkPolicy"
	//ConfigMap -
	ConfigMap = "ConfigMap"
	//Secret -
	Secret = "Secret"
	//ServiceAccount -
	ServiceAccount = "ServiceAccount"
	//RoleBinding -
	RoleBinding = "RoleBinding"
	//HorizontalPodAutoscaler -
	HorizontalPodAutoscaler = "HorizontalPodAutoscaler"
	//Role -
	Role = "Role"
	//Gateway -
	Gateway = "Gateway"
	//HTTPRoute -
	HTTPRoute = "HTTPRoute"
	Rollout   = "Rollout"

	//ClusterRoleBinding -
	ClusterRoleBinding = "ClusterRoleBinding"
)

const (
	//APIVersionSecret -
	APIVersionSecret = "v1"
	//APIVersionConfigMap -
	APIVersionConfigMap = "v1"
	//APIVersionServiceAccount -
	APIVersionServiceAccount = "v1"
	//APIVersionPersistentVolumeClaim -
	APIVersionPersistentVolumeClaim = "v1"
	//APIVersionStatefulSet -
	APIVersionStatefulSet = "apps/v1"
	//APIVersionDeployment -
	APIVersionDeployment = "apps/v1"
	//APIVersionJob -
	APIVersionJob = "batch/v1"
	//APIVersionCronJob -
	APIVersionCronJob = "batch/v1"
	//APIVersionBetaCronJob -
	APIVersionBetaCronJob = "batch/v1beta1"
	//APIVersionService -
	APIVersionService = "v1"
	//APIVersionHorizontalPodAutoscaler -q
	APIVersionHorizontalPodAutoscaler = "autoscaling/v2"
	//APIVersionGateway -
	APIVersionGateway = "gateway.networking.k8s.io/v1beta1"
	//APIVersionHTTPRoute -
	APIVersionHTTPRoute = "gateway.networking.k8s.io/v1beta1"
	//APIVersionRollout -
	APIVersionRollout = "rollouts.kruise.io/v1alpha1"
)
