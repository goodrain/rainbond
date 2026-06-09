package model

// PodDetail -
type PodDetail struct {
	Name           string          `json:"name,omitempty"`
	Node           string          `json:"node,omitempty"`
	StartTime      string          `json:"start_time,omitempty"`
	Status         *PodStatus      `json:"status,omitempty"`
	IP             string          `json:"ip,omitempty"`
	InitContainers []*PodContainer `json:"init_containers,omitempty"`
	Containers     []*PodContainer `json:"containers,omitempty"`
	Events         []*PodEvent     `json:"events,omitempty"`
}

// PodStatus -
type PodStatus struct {
	Type    int    `json:"type,omitempty"`
	TypeStr string `json:"type_str,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	Advice  string `json:"advice,omitempty"`
}

// PodContainer -
type PodContainer struct {
	Image       string `json:"image,omitempty"`
	State       string `json:"state,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Started     string `json:"started,omitempty"`
	LimitMemory string `json:"limit_memory,omitempty"`
	LimitCPU    string `json:"limit_cpu,omitempty"`
}

// PodEvent -
type PodEvent struct {
	Type    string `json:"type,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Age     string `json:"age,omitempty"`
	Message string `json:"message,omitempty"`
}

// PodExecRequest is the request body for the one-shot pod exec endpoint.
type PodExecRequest struct {
	// Container is the target container name. Optional; defaults to the
	// component's main container when empty.
	Container string `json:"container"`
	// Command is the command to execute, e.g. ["sh", "-c", "echo hello"].
	// Required and must be non-empty.
	Command []string `json:"command" validate:"command|required"`
	// TimeoutSeconds bounds how long the exec may run. Optional; clamped to
	// a sane default and maximum on the server side.
	TimeoutSeconds int `json:"timeout_seconds"`
}

// PodExecResult is the response body for the one-shot pod exec endpoint.
type PodExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	// Truncated is true when stdout or stderr exceeded the output cap and
	// was trimmed.
	Truncated bool `json:"truncated"`
}
