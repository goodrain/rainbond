package constants

// Shared helper identities keep runtime defaults and reference protection aligned.
const (
	// CNBBuilderImageName is the internal CNB builder image and version.
	CNBBuilderImageName = "ubuntu-noble-builder:0.0.98"
	// CNBRunImageName is the internal CNB run image and version.
	CNBRunImageName = "ubuntu-noble-run:0.0.73"
	// PHPCNBBuilderImageName is the PHP builder image and version.
	PHPCNBBuilderImageName = "builder-jammy-full:0.3.613"
	// PHPCNBRunImageName is the PHP run image and version.
	PHPCNBRunImageName = "run-jammy-full:0.1.141"
	// VMQCOW2ConverterImage is the default VM conversion executor.
	VMQCOW2ConverterImage = "quay.io/kubevirt/cdi-importer:v1.65.0"
	// VMHTTPArtifactImage is the default VM artifact server image.
	VMHTTPArtifactImage = "registry.cn-hangzhou.aliyuncs.com/zhangqihang/nginx:1.25-alpine"
)
