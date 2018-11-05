package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

var (
	buildno       = "1"
	user          = "vincent"
	empty         = "\x00"
	testCacheFrom = "someotherimage:latest"
	target        = "intermediateimage"
	network       = "buildnetwork"
)

func TestMarshalBuild(t *testing.T) {
	builds := []struct {
		build    Build
		expected string
	}{
		{
			expected: `{}
`,
		},
		{
			build: Build{
				Context: ".",
			},
			expected: `context: .
`,
		},
		{
			build: Build{
				Context:    ".",
				Dockerfile: "alternate",
			},
			expected: `context: .
dockerfile: alternate
`,
		},
		{
			build: Build{
				Context:    ".",
				Dockerfile: "alternate",
				Args: map[string]*string{
					"buildno": &buildno,
					"user":    &user,
				},
				CacheFrom: []*string{
					&testCacheFrom,
				},
				Labels: map[string]*string{
					"buildno": &buildno,
					"user":    &user,
				},
				Target:  target,
				Network: network,
			},
			expected: `args:
  buildno: "1"
  user: vincent
cache_from:
- someotherimage:latest
context: .
dockerfile: alternate
labels:
  buildno: "1"
  user: vincent
network: buildnetwork
target: intermediateimage
`,
		},
	}
	for _, build := range builds {
		bytes, err := yaml.Marshal(build.build)
		assert.Nil(t, err)
		assert.Equal(t, build.expected, string(bytes), "should be equal")
	}
}

func TestUnmarshalBuild(t *testing.T) {
	builds := []struct {
		yaml     string
		expected *Build
	}{
		{
			yaml: `.`,
			expected: &Build{
				Context: ".",
			},
		},
		{
			yaml: `context: .`,
			expected: &Build{
				Context: ".",
			},
		},
		{
			yaml: `context: .
dockerfile: alternate`,
			expected: &Build{
				Context:    ".",
				Dockerfile: "alternate",
			},
		},
		{
			yaml: `context: .
dockerfile: alternate
args:
  buildno: 1
  user: vincent
cache_from:
  - someotherimage:latest
labels:
  buildno: "1"
  user: vincent
target: intermediateimage
network: buildnetwork
`,
			expected: &Build{
				Context:    ".",
				Dockerfile: "alternate",
				Args: map[string]*string{
					"buildno": &buildno,
					"user":    &user,
				},
				CacheFrom: []*string{
					&testCacheFrom,
				},
				Labels: map[string]*string{
					"buildno": &buildno,
					"user":    &user,
				},
				Target:  target,
				Network: network,
			},
		},
		{
			yaml: `context: .
args:
  - buildno
  - user`,
			expected: &Build{
				Context: ".",
				Args: map[string]*string{
					"buildno": &empty,
					"user":    &empty,
				},
			},
		},
		{
			yaml: `context: .
args:
  - buildno=1
  - user=vincent`,
			expected: &Build{
				Context: ".",
				Args: map[string]*string{
					"buildno": &buildno,
					"user":    &user,
				},
			},
		},
	}
	for _, build := range builds {
		actual := &Build{}
		err := yaml.Unmarshal([]byte(build.yaml), actual)
		assert.Nil(t, err)
		assert.Equal(t, build.expected, actual, "should be equal")
	}
}
