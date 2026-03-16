package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetStorageHandlerSingleton(t *testing.T) {
	h1 := GetStorageHandler()
	h2 := GetStorageHandler()
	assert.NotNil(t, h1)
	assert.Equal(t, h1, h2)
}

func TestStorageClassInfoFields(t *testing.T) {
	info := StorageClassInfo{
		Name:                 "test-sc",
		Provisioner:          "rancher.io/local-path",
		IsDefault:            true,
		ReclaimPolicy:        "Delete",
		VolumeBindingMode:    "WaitForFirstConsumer",
		AllowVolumeExpansion: true,
		PVCount:              3,
	}
	assert.Equal(t, "test-sc", info.Name)
	assert.True(t, info.IsDefault)
	assert.Equal(t, 3, info.PVCount)
}
