package coquelicot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrepareDir(t *testing.T) {
	assert := assert.New(t)
	root := "dummy/root_storage"

	dm, err := createDir(root, "image")
	assert.Nil(err)
	assert.Equal(root, dm.Root)

	dm, err = checkDir(root, "/image/2014/2a/q1b12")
	assert.Nil(err)
}
