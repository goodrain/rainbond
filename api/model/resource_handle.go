package model

import (
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

//LabelResource -
type LabelResource struct {
	Workloads WorkLoadsResource   `json:"workloads,omitempty"`
	Others    OtherResource       `json:"others,omitempty"`
	UnSupport map[string][]string `json:"un_support"`
	Status    string              `json:"status"`
}

//LabelWorkloadsResourceProcess -
type LabelWorkloadsResourceProcess struct {
	Deployments  map[string][]string `json:"deployments,omitempty"`
	Jobs         map[string][]string `json:"jobs,omitempty"`
	CronJobs     map[string][]string `json:"cronJobs,omitempty"`
	StateFulSets map[string][]string `json:"stateFulSets,omitempty"`
}

//LabelOthersResourceProcess -
type LabelOthersResourceProcess struct {
	Services                 map[string][]string `json:"services,omitempty"`
	PVC                      map[string][]string `json:"PVC,omitempty"`
	Ingresses                map[string][]string `json:"ingresses,omitempty"`
	NetworkPolicies          map[string][]string `json:"networkPolicies,omitempty"`
	ConfigMaps               map[string][]string `json:"configMaps,omitempty"`
	Secrets                  map[string][]string `json:"secrets,omitempty"`
	ServiceAccounts          map[string][]string `json:"serviceAccounts,omitempty"`
	RoleBindings             map[string][]string `json:"roleBindings,omitempty"`
	HorizontalPodAutoscalers map[string][]string `json:"horizontalPodAutoscalers,omitempty"`
	Roles                    map[string][]string `json:"roles,omitempty"`
}

//YamlResourceParameter -
type YamlResourceParameter struct {
	ComponentsCR *[]ConvertResource
	Basic        BasicManagement
	Template     corev1.PodTemplateSpec
	Namespace    string
	Name         string
	RsLabel      map[string]string
	CMs          []corev1.ConfigMap
	HPAs         []autoscalingv1.HorizontalPodAutoscaler
}

//K8sResourceObject -
type K8sResourceObject struct {
	FileName       string
	BuildResources []BuildResource
	Error          string
}

//WorkLoadsResource -
type WorkLoadsResource struct {
	Deployments  []string `json:"Deployment,omitempty"`
	Jobs         []string `json:"Job,omitempty"`
	CronJobs     []string `json:"CronJob,omitempty"`
	StateFulSets []string `json:"StatefulSet,omitempty"`
}

//BuildResource -
type BuildResource struct {
	Resource      *unstructured.Unstructured
	State         int
	ErrorOverview string
	Dri           dynamic.ResourceInterface
	DC            dynamic.Interface
	GVK           *schema.GroupVersionKind
}

//OtherResource -
type OtherResource struct {
	Services                 []string `json:"Service,omitempty"`
	PVC                      []string `json:"PVC,omitempty"`
	Ingresses                []string `json:"Ingress,omitempty"`
	NetworkPolicies          []string `json:"NetworkPolicie,omitempty"`
	ConfigMaps               []string `json:"ConfigMap,omitempty"`
	Secrets                  []string `json:"Secret,omitempty"`
	ServiceAccounts          []string `json:"ServiceAccount,omitempty"`
	RoleBindings             []string `json:"RoleBinding,omitempty"`
	HorizontalPodAutoscalers []string `json:"HorizontalPodAutoscaler,omitempty"`
	Roles                    []string `json:"Role,omitempty"`
}
