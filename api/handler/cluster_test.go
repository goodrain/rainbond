package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildShellPodGenerateNameLowercasesRegionName(t *testing.T) {
	generateName := buildShellPodGenerateName("GXZY-K8S")

	assert.Equal(t, "shell-gxzy-k8s-", generateName)
}
