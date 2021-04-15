package helm

import (
	"github.com/goodrain/rainbond/util"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRepoAdd(t *testing.T) {
	repo := NewRepo(util.NewUUID(),
		"https://openchart.goodrain.com/goodrain/rainbond",
		"", "",
		"/tmp/helm/repo/repositories.yaml",
		"/tmp/helm/cache")
	err := repo.Add()
	assert.Nil(t, err)
}
