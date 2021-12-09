package model

import "github.com/goodrain/rainbond/worker/server/pb"

// AppPort -
type AppPort struct {
	ServiceID      string `json:"service_id" validate:"required"`
	ContainerPort  int    `json:"container_port" validate:"required"`
	PortAlias      string `json:"port_alias" validate:"required"`
	K8sServiceName string `json:"k8s_service_name" validate:"required"`
}

// AppStatus -
type AppStatus struct {
	AppID      string                `json:"app_id"`
	AppName    string                `json:"app_name"`
	Status     string                `json:"status"`
	CPU        *int64                `json:"cpu"`
	GPU        *int64                `json:"gpu"`
	Memory     *int64                `json:"memory"`
	Disk       int64                 `json:"disk"`
	Phase      string                `json:"phase"`
	Version    string                `json:"version"`
	Overrides  []string              `json:"overrides"`
	Conditions []*AppStatusCondition `json:"conditions"`
	K8sApp     string                `json:"k8s_app"`
}

// AppStatusCondition is the conditon of app status.
type AppStatusCondition struct {
	Type    string `json:"type"`
	Status  bool   `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// AppService -
type AppService struct {
	ServiceName string                `json:"service_name"`
	Address     string                `json:"address"`
	Ports       []*pb.AppService_Port `json:"ports"`
	OldPods     []*AppPod             `json:"oldPods"`
	Pods        []*AppPod             `json:"pods"`
}

// ByServiceName implements sort.Interface for []*AppService based on
// the ServiceName field.
type ByServiceName []*AppService

func (a ByServiceName) Len() int           { return len(a) }
func (a ByServiceName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByServiceName) Less(i, j int) bool { return a[i].ServiceName < a[j].ServiceName }

// AppPod -
type AppPod struct {
	PodName   string `json:"pod_name"`
	PodStatus string `json:"pod_status"`
}

// ByPodName implements sort.Interface for []*AppPod based on
// the PodName field.
type ByPodName []*AppPod

func (a ByPodName) Len() int           { return len(a) }
func (a ByPodName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByPodName) Less(i, j int) bool { return a[i].PodName < a[j].PodName }
