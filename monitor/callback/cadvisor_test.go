package callback

import (
	"gopkg.in/yaml.v2"
	"testing"
)

func TestCadvisorYaml(t *testing.T) {
	c := &Cadvisor{}
	cfg := c.toScrape()
	_, err := yaml.Marshal(cfg)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
}
