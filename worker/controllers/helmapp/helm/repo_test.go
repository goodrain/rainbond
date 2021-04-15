package helm

import (
	"github.com/goodrain/rainbond/util"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRepoAdd(t *testing.T) {
	repo := NewRepo(
		"/tmp/helm/repo/repositories.yaml",
		"/tmp/helm/cache")
	err := repo.Add(util.NewUUID(), "https://openchart.goodrain.com/goodrain/rainbond", "", "")
	assert.Nil(t, err)
}
