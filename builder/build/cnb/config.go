package cnb

import "github.com/goodrain/rainbond/util"

const (
	// DefaultCNBBuilder is the default CNB builder image
	DefaultCNBBuilder = "registry.cn-hangzhou.aliyuncs.com/goodrain/ubuntu-noble-builder:latest"
	// DefaultCNBRunImage is the default CNB run image
	DefaultCNBRunImage = "registry.cn-hangzhou.aliyuncs.com/goodrain/ubuntu-noble-run:0.0.50"
	// CNBLifecycleCreatorPath is the path to the lifecycle creator binary in builder image
	CNBLifecycleCreatorPath = "/lifecycle/creator"
)

// GetCNBBuilderImage returns the CNB builder image from environment or default
func GetCNBBuilderImage() string {
	return util.GetenvDefault("CNB_BUILDER_IMAGE", DefaultCNBBuilder)
}

// GetCNBRunImage returns the CNB run image from environment or default
func GetCNBRunImage() string {
	return util.GetenvDefault("CNB_RUN_IMAGE", DefaultCNBRunImage)
}
