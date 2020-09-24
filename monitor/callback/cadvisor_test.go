package callback

import (
	"regexp"
	"testing"

	yaml "gopkg.in/yaml.v2"
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

func TestRegex(t *testing.T) {
	match, err := regexp.Compile("k8s_(.*)_(.*)_(.*)_(.*)_(.*)")
	if err != nil {
		t.Fatal(err)
	}
	result := match.FindStringSubmatch("k8s_827d488f787c4c109013e8119d42078d_827d488f787c4c109013e8119d42078d-deployment-85f9fd5db6-gmg66_991fe537972e45378acb7920bde7599b_f9eed2bb-311a-48a5-8ea9-4e2df77fc12e_0")
	t.Log(result)
	result = match.FindStringSubmatch("k8s_d89ffc075ca74476b6040c8e8bae9756_grae9756-0_3be96e95700a480c9b37c6ef5daf3566_dfe3018b-d0ef-45de-85d6-62f01360eabf_0")
	t.Log(result)
}
