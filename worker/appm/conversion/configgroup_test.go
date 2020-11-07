package conversion

import (
	"github.com/goodrain/rainbond/db/model"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConfigGroupParseVariable(t *testing.T) {
	items := []*model.ConfigGroupItem{
		{
			ItemKey: "foo",
			ItemValue: "bar",
		},
		{
			ItemKey: "foo2",
			ItemValue: "${foo}",
		},
	}
	c := &configGroup{}
	variables := c.parseVariable(items)

	assert.Equal(t, "bar", string(variables["foo2"]))
}
