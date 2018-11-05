package convert

import (
	"testing"

	composetypes "github.com/docker/cli/cli/compose/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/fs"
)

func TestNamespaceScope(t *testing.T) {
	scoped := Namespace{name: "foo"}.Scope("bar")
	assert.Check(t, is.Equal("foo_bar", scoped))
}

func TestAddStackLabel(t *testing.T) {
	labels := map[string]string{
		"something": "labeled",
	}
	actual := AddStackLabel(Namespace{name: "foo"}, labels)
	expected := map[string]string{
		"something":    "labeled",
		LabelNamespace: "foo",
	}
	assert.Check(t, is.DeepEqual(expected, actual))
}

func TestNetworks(t *testing.T) {
	namespace := Namespace{name: "foo"}
	serviceNetworks := map[string]struct{}{
		"normal":        {},
		"outside":       {},
		"default":       {},
		"attachablenet": {},
		"named":         {},
	}
	source := networkMap{
		"normal": composetypes.NetworkConfig{
			Driver: "overlay",
			DriverOpts: map[string]string{
				"opt": "value",
			},
			Ipam: composetypes.IPAMConfig{
				Driver: "driver",
				Config: []*composetypes.IPAMPool{
					{
						Subnet: "10.0.0.0",
					},
				},
			},
			Labels: map[string]string{
				"something": "labeled",
			},
		},
		"outside": composetypes.NetworkConfig{
			External: composetypes.External{External: true},
			Name:     "special",
		},
		"attachablenet": composetypes.NetworkConfig{
			Driver:     "overlay",
			Attachable: true,
		},
		"named": composetypes.NetworkConfig{
			Name: "othername",
		},
	}
	expected := map[string]types.NetworkCreate{
		"foo_default": {
			Labels: map[string]string{
				LabelNamespace: "foo",
			},
		},
		"foo_normal": {
			Driver: "overlay",
			IPAM: &network.IPAM{
				Driver: "driver",
				Config: []network.IPAMConfig{
					{
						Subnet: "10.0.0.0",
					},
				},
			},
			Options: map[string]string{
				"opt": "value",
			},
			Labels: map[string]string{
				LabelNamespace: "foo",
				"something":    "labeled",
			},
		},
		"foo_attachablenet": {
			Driver:     "overlay",
			Attachable: true,
			Labels: map[string]string{
				LabelNamespace: "foo",
			},
		},
		"othername": {
			Labels: map[string]string{LabelNamespace: "foo"},
		},
	}

	networks, externals := Networks(namespace, source, serviceNetworks)
	assert.DeepEqual(t, expected, networks)
	assert.DeepEqual(t, []string{"special"}, externals)
}

func TestSecrets(t *testing.T) {
	namespace := Namespace{name: "foo"}

	secretText := "this is the first secret"
	secretFile := fs.NewFile(t, "convert-secrets", fs.WithContent(secretText))
	defer secretFile.Remove()

	source := map[string]composetypes.SecretConfig{
		"one": {
			File:   secretFile.Path(),
			Labels: map[string]string{"monster": "mash"},
		},
		"ext": {
			External: composetypes.External{
				External: true,
			},
		},
	}

	specs, err := Secrets(namespace, source)
	assert.NilError(t, err)
	assert.Assert(t, is.Len(specs, 1))
	secret := specs[0]
	assert.Check(t, is.Equal("foo_one", secret.Name))
	assert.Check(t, is.DeepEqual(map[string]string{
		"monster":      "mash",
		LabelNamespace: "foo",
	}, secret.Labels))
	assert.Check(t, is.DeepEqual([]byte(secretText), secret.Data))
}

func TestConfigs(t *testing.T) {
	namespace := Namespace{name: "foo"}

	configText := "this is the first config"
	configFile := fs.NewFile(t, "convert-configs", fs.WithContent(configText))
	defer configFile.Remove()

	source := map[string]composetypes.ConfigObjConfig{
		"one": {
			File:   configFile.Path(),
			Labels: map[string]string{"monster": "mash"},
		},
		"ext": {
			External: composetypes.External{
				External: true,
			},
		},
	}

	specs, err := Configs(namespace, source)
	assert.NilError(t, err)
	assert.Assert(t, is.Len(specs, 1))
	config := specs[0]
	assert.Check(t, is.Equal("foo_one", config.Name))
	assert.Check(t, is.DeepEqual(map[string]string{
		"monster":      "mash",
		LabelNamespace: "foo",
	}, config.Labels))
	assert.Check(t, is.DeepEqual([]byte(configText), config.Data))
}
