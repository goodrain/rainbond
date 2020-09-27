package model

// AppPort -
type AppPort struct {
	ServiceID      string `json:"service_id" validate:"required"`
	ContainerPort  int    `json:"container_port" validate:"required"`
	PortAlias      string `json:"port_alias" validate:"required"`
	K8sServiceName string `json:"k8s_service_name" validate:"required"`
}
