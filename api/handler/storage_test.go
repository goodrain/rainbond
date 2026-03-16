package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetStorageOverview(t *testing.T) {
	handler := NewStorageHandler()
	overview, err := handler.GetStorageOverview()
	assert.NoError(t, err)
	assert.NotNil(t, overview)
}

func TestListStorageClasses(t *testing.T) {
	handler := NewStorageHandler()
	classes, err := handler.ListStorageClasses()
	assert.NoError(t, err)
	assert.NotNil(t, classes)
}

func TestListPersistentVolumes(t *testing.T) {
	handler := NewStorageHandler()
	pvs, err := handler.ListPersistentVolumes()
	assert.NoError(t, err)
	assert.NotNil(t, pvs)
}
