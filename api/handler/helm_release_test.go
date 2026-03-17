package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetHelmReleaseHandlerSingleton(t *testing.T) {
	h1 := GetHelmReleaseHandler()
	h2 := GetHelmReleaseHandler()
	assert.NotNil(t, h1)
	assert.Equal(t, h1, h2)
}
