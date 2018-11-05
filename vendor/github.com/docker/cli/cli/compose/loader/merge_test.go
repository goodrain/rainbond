package loader

import (
	"testing"

	"github.com/docker/cli/cli/compose/types"
	"gotest.tools/assert"
)

func TestLoadTwoDifferentVersion(t *testing.T) {
	configDetails := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "base.yml", Config: map[string]interface{}{
				"version": "3.1",
			}},
			{Filename: "override.yml", Config: map[string]interface{}{
				"version": "3.4",
			}},
		},
	}
	_, err := Load(configDetails)
	assert.Error(t, err, "version mismatched between two composefiles : 3.1 and 3.4")
}

func TestLoadLogging(t *testing.T) {
	loggingCases := []struct {
		name            string
		loggingBase     map[string]interface{}
		loggingOverride map[string]interface{}
		expected        *types.LoggingConfig
	}{
		{
			name: "no_override_driver",
			loggingBase: map[string]interface{}{
				"logging": map[string]interface{}{
					"driver": "json-file",
					"options": map[string]interface{}{
						"frequency": "2000",
						"timeout":   "23",
					},
				},
			},
			loggingOverride: map[string]interface{}{
				"logging": map[string]interface{}{
					"options": map[string]interface{}{
						"timeout":      "360",
						"pretty-print": "on",
					},
				},
			},
			expected: &types.LoggingConfig{
				Driver: "json-file",
				Options: map[string]string{
					"frequency":    "2000",
					"timeout":      "360",
					"pretty-print": "on",
				},
			},
		},
		{
			name: "override_driver",
			loggingBase: map[string]interface{}{
				"logging": map[string]interface{}{
					"driver": "json-file",
					"options": map[string]interface{}{
						"frequency": "2000",
						"timeout":   "23",
					},
				},
			},
			loggingOverride: map[string]interface{}{
				"logging": map[string]interface{}{
					"driver": "syslog",
					"options": map[string]interface{}{
						"timeout":      "360",
						"pretty-print": "on",
					},
				},
			},
			expected: &types.LoggingConfig{
				Driver: "syslog",
				Options: map[string]string{
					"timeout":      "360",
					"pretty-print": "on",
				},
			},
		},
		{
			name: "no_base_driver",
			loggingBase: map[string]interface{}{
				"logging": map[string]interface{}{
					"options": map[string]interface{}{
						"frequency": "2000",
						"timeout":   "23",
					},
				},
			},
			loggingOverride: map[string]interface{}{
				"logging": map[string]interface{}{
					"driver": "json-file",
					"options": map[string]interface{}{
						"timeout":      "360",
						"pretty-print": "on",
					},
				},
			},
			expected: &types.LoggingConfig{
				Driver: "json-file",
				Options: map[string]string{
					"frequency":    "2000",
					"timeout":      "360",
					"pretty-print": "on",
				},
			},
		},
		{
			name: "no_driver",
			loggingBase: map[string]interface{}{
				"logging": map[string]interface{}{
					"options": map[string]interface{}{
						"frequency": "2000",
						"timeout":   "23",
					},
				},
			},
			loggingOverride: map[string]interface{}{
				"logging": map[string]interface{}{
					"options": map[string]interface{}{
						"timeout":      "360",
						"pretty-print": "on",
					},
				},
			},
			expected: &types.LoggingConfig{
				Options: map[string]string{
					"frequency":    "2000",
					"timeout":      "360",
					"pretty-print": "on",
				},
			},
		},
		{
			name: "no_override_options",
			loggingBase: map[string]interface{}{
				"logging": map[string]interface{}{
					"driver": "json-file",
					"options": map[string]interface{}{
						"frequency": "2000",
						"timeout":   "23",
					},
				},
			},
			loggingOverride: map[string]interface{}{
				"logging": map[string]interface{}{
					"driver": "syslog",
				},
			},
			expected: &types.LoggingConfig{
				Driver: "syslog",
			},
		},
		{
			name:        "no_base",
			loggingBase: map[string]interface{}{},
			loggingOverride: map[string]interface{}{
				"logging": map[string]interface{}{
					"driver": "json-file",
					"options": map[string]interface{}{
						"frequency": "2000",
					},
				},
			},
			expected: &types.LoggingConfig{
				Driver: "json-file",
				Options: map[string]string{
					"frequency": "2000",
				},
			},
		},
	}

	for _, tc := range loggingCases {
		t.Run(tc.name, func(t *testing.T) {
			configDetails := types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{
					{
						Filename: "base.yml",
						Config: map[string]interface{}{
							"version": "3.4",
							"services": map[string]interface{}{
								"foo": tc.loggingBase,
							},
						},
					},
					{
						Filename: "override.yml",
						Config: map[string]interface{}{
							"version": "3.4",
							"services": map[string]interface{}{
								"foo": tc.loggingOverride,
							},
						},
					},
				},
			}
			config, err := Load(configDetails)
			assert.NilError(t, err)
			assert.DeepEqual(t, &types.Config{
				Filename: "base.yml",
				Version:  "3.4",
				Services: []types.ServiceConfig{
					{
						Name:        "foo",
						Logging:     tc.expected,
						Environment: types.MappingWithEquals{},
					},
				},
				Networks: map[string]types.NetworkConfig{},
				Volumes:  map[string]types.VolumeConfig{},
				Secrets:  map[string]types.SecretConfig{},
				Configs:  map[string]types.ConfigObjConfig{},
			}, config)
		})
	}
}

func TestLoadMultipleServicePorts(t *testing.T) {
	portsCases := []struct {
		name         string
		portBase     map[string]interface{}
		portOverride map[string]interface{}
		expected     []types.ServicePortConfig
	}{
		{
			name: "no_override",
			portBase: map[string]interface{}{
				"ports": []interface{}{
					"8080:80",
				},
			},
			portOverride: map[string]interface{}{},
			expected: []types.ServicePortConfig{
				{
					Mode:      "ingress",
					Published: 8080,
					Target:    80,
					Protocol:  "tcp",
				},
			},
		},
		{
			name: "override_different_published",
			portBase: map[string]interface{}{
				"ports": []interface{}{
					"8080:80",
				},
			},
			portOverride: map[string]interface{}{
				"ports": []interface{}{
					"8081:80",
				},
			},
			expected: []types.ServicePortConfig{
				{
					Mode:      "ingress",
					Published: 8080,
					Target:    80,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					Published: 8081,
					Target:    80,
					Protocol:  "tcp",
				},
			},
		},
		{
			name: "override_same_published",
			portBase: map[string]interface{}{
				"ports": []interface{}{
					"8080:80",
				},
			},
			portOverride: map[string]interface{}{
				"ports": []interface{}{
					"8080:81",
				},
			},
			expected: []types.ServicePortConfig{
				{
					Mode:      "ingress",
					Published: 8080,
					Target:    81,
					Protocol:  "tcp",
				},
			},
		},
	}

	for _, tc := range portsCases {
		t.Run(tc.name, func(t *testing.T) {
			configDetails := types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{
					{
						Filename: "base.yml",
						Config: map[string]interface{}{
							"version": "3.4",
							"services": map[string]interface{}{
								"foo": tc.portBase,
							},
						},
					},
					{
						Filename: "override.yml",
						Config: map[string]interface{}{
							"version": "3.4",
							"services": map[string]interface{}{
								"foo": tc.portOverride,
							},
						},
					},
				},
			}
			config, err := Load(configDetails)
			assert.NilError(t, err)
			assert.DeepEqual(t, &types.Config{
				Filename: "base.yml",
				Version:  "3.4",
				Services: []types.ServiceConfig{
					{
						Name:        "foo",
						Ports:       tc.expected,
						Environment: types.MappingWithEquals{},
					},
				},
				Networks: map[string]types.NetworkConfig{},
				Volumes:  map[string]types.VolumeConfig{},
				Secrets:  map[string]types.SecretConfig{},
				Configs:  map[string]types.ConfigObjConfig{},
			}, config)
		})
	}
}

func TestLoadMultipleSecretsConfig(t *testing.T) {
	portsCases := []struct {
		name           string
		secretBase     map[string]interface{}
		secretOverride map[string]interface{}
		expected       []types.ServiceSecretConfig
	}{
		{
			name: "no_override",
			secretBase: map[string]interface{}{
				"secrets": []interface{}{
					"my_secret",
				},
			},
			secretOverride: map[string]interface{}{},
			expected: []types.ServiceSecretConfig{
				{
					Source: "my_secret",
				},
			},
		},
		{
			name: "override_simple",
			secretBase: map[string]interface{}{
				"secrets": []interface{}{
					"foo_secret",
				},
			},
			secretOverride: map[string]interface{}{
				"secrets": []interface{}{
					"bar_secret",
				},
			},
			expected: []types.ServiceSecretConfig{
				{
					Source: "bar_secret",
				},
				{
					Source: "foo_secret",
				},
			},
		},
		{
			name: "override_same_source",
			secretBase: map[string]interface{}{
				"secrets": []interface{}{
					"foo_secret",
					map[string]interface{}{
						"source": "bar_secret",
						"target": "waw_secret",
					},
				},
			},
			secretOverride: map[string]interface{}{
				"secrets": []interface{}{
					map[string]interface{}{
						"source": "bar_secret",
						"target": "bof_secret",
					},
					map[string]interface{}{
						"source": "baz_secret",
						"target": "waw_secret",
					},
				},
			},
			expected: []types.ServiceSecretConfig{
				{
					Source: "bar_secret",
					Target: "bof_secret",
				},
				{
					Source: "baz_secret",
					Target: "waw_secret",
				},
				{
					Source: "foo_secret",
				},
			},
		},
	}

	for _, tc := range portsCases {
		t.Run(tc.name, func(t *testing.T) {
			configDetails := types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{
					{
						Filename: "base.yml",
						Config: map[string]interface{}{
							"version": "3.4",
							"services": map[string]interface{}{
								"foo": tc.secretBase,
							},
						},
					},
					{
						Filename: "override.yml",
						Config: map[string]interface{}{
							"version": "3.4",
							"services": map[string]interface{}{
								"foo": tc.secretOverride,
							},
						},
					},
				},
			}
			config, err := Load(configDetails)
			assert.NilError(t, err)
			assert.DeepEqual(t, &types.Config{
				Filename: "base.yml",
				Version:  "3.4",
				Services: []types.ServiceConfig{
					{
						Name:        "foo",
						Secrets:     tc.expected,
						Environment: types.MappingWithEquals{},
					},
				},
				Networks: map[string]types.NetworkConfig{},
				Volumes:  map[string]types.VolumeConfig{},
				Secrets:  map[string]types.SecretConfig{},
				Configs:  map[string]types.ConfigObjConfig{},
			}, config)
		})
	}
}

func TestLoadMultipleConfigobjsConfig(t *testing.T) {
	portsCases := []struct {
		name           string
		configBase     map[string]interface{}
		configOverride map[string]interface{}
		expected       []types.ServiceConfigObjConfig
	}{
		{
			name: "no_override",
			configBase: map[string]interface{}{
				"configs": []interface{}{
					"my_config",
				},
			},
			configOverride: map[string]interface{}{},
			expected: []types.ServiceConfigObjConfig{
				{
					Source: "my_config",
				},
			},
		},
		{
			name: "override_simple",
			configBase: map[string]interface{}{
				"configs": []interface{}{
					"foo_config",
				},
			},
			configOverride: map[string]interface{}{
				"configs": []interface{}{
					"bar_config",
				},
			},
			expected: []types.ServiceConfigObjConfig{
				{
					Source: "bar_config",
				},
				{
					Source: "foo_config",
				},
			},
		},
		{
			name: "override_same_source",
			configBase: map[string]interface{}{
				"configs": []interface{}{
					"foo_config",
					map[string]interface{}{
						"source": "bar_config",
						"target": "waw_config",
					},
				},
			},
			configOverride: map[string]interface{}{
				"configs": []interface{}{
					map[string]interface{}{
						"source": "bar_config",
						"target": "bof_config",
					},
					map[string]interface{}{
						"source": "baz_config",
						"target": "waw_config",
					},
				},
			},
			expected: []types.ServiceConfigObjConfig{
				{
					Source: "bar_config",
					Target: "bof_config",
				},
				{
					Source: "baz_config",
					Target: "waw_config",
				},
				{
					Source: "foo_config",
				},
			},
		},
	}

	for _, tc := range portsCases {
		t.Run(tc.name, func(t *testing.T) {
			configDetails := types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{
					{
						Filename: "base.yml",
						Config: map[string]interface{}{
							"version": "3.4",
							"services": map[string]interface{}{
								"foo": tc.configBase,
							},
						},
					},
					{
						Filename: "override.yml",
						Config: map[string]interface{}{
							"version": "3.4",
							"services": map[string]interface{}{
								"foo": tc.configOverride,
							},
						},
					},
				},
			}
			config, err := Load(configDetails)
			assert.NilError(t, err)
			assert.DeepEqual(t, &types.Config{
				Filename: "base.yml",
				Version:  "3.4",
				Services: []types.ServiceConfig{
					{
						Name:        "foo",
						Configs:     tc.expected,
						Environment: types.MappingWithEquals{},
					},
				},
				Networks: map[string]types.NetworkConfig{},
				Volumes:  map[string]types.VolumeConfig{},
				Secrets:  map[string]types.SecretConfig{},
				Configs:  map[string]types.ConfigObjConfig{},
			}, config)
		})
	}
}

func TestLoadMultipleUlimits(t *testing.T) {
	ulimitCases := []struct {
		name           string
		ulimitBase     map[string]interface{}
		ulimitOverride map[string]interface{}
		expected       map[string]*types.UlimitsConfig
	}{
		{
			name: "no_override",
			ulimitBase: map[string]interface{}{
				"ulimits": map[string]interface{}{
					"noproc": 65535,
				},
			},
			ulimitOverride: map[string]interface{}{},
			expected: map[string]*types.UlimitsConfig{
				"noproc": {
					Single: 65535,
				},
			},
		},
		{
			name: "override_simple",
			ulimitBase: map[string]interface{}{
				"ulimits": map[string]interface{}{
					"noproc": 65535,
				},
			},
			ulimitOverride: map[string]interface{}{
				"ulimits": map[string]interface{}{
					"noproc": 44444,
				},
			},
			expected: map[string]*types.UlimitsConfig{
				"noproc": {
					Single: 44444,
				},
			},
		},
		{
			name: "override_different_notation",
			ulimitBase: map[string]interface{}{
				"ulimits": map[string]interface{}{
					"nofile": map[string]interface{}{
						"soft": 11111,
						"hard": 99999,
					},
					"noproc": 44444,
				},
			},
			ulimitOverride: map[string]interface{}{
				"ulimits": map[string]interface{}{
					"nofile": 55555,
					"noproc": map[string]interface{}{
						"soft": 22222,
						"hard": 33333,
					},
				},
			},
			expected: map[string]*types.UlimitsConfig{
				"noproc": {
					Soft: 22222,
					Hard: 33333,
				},
				"nofile": {
					Single: 55555,
				},
			},
		},
	}

	for _, tc := range ulimitCases {
		t.Run(tc.name, func(t *testing.T) {
			configDetails := types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{
					{
						Filename: "base.yml",
						Config: map[string]interface{}{
							"version": "3.4",
							"services": map[string]interface{}{
								"foo": tc.ulimitBase,
							},
						},
					},
					{
						Filename: "override.yml",
						Config: map[string]interface{}{
							"version": "3.4",
							"services": map[string]interface{}{
								"foo": tc.ulimitOverride,
							},
						},
					},
				},
			}
			config, err := Load(configDetails)
			assert.NilError(t, err)
			assert.DeepEqual(t, &types.Config{
				Filename: "base.yml",
				Version:  "3.4",
				Services: []types.ServiceConfig{
					{
						Name:        "foo",
						Ulimits:     tc.expected,
						Environment: types.MappingWithEquals{},
					},
				},
				Networks: map[string]types.NetworkConfig{},
				Volumes:  map[string]types.VolumeConfig{},
				Secrets:  map[string]types.SecretConfig{},
				Configs:  map[string]types.ConfigObjConfig{},
			}, config)
		})
	}
}

func TestLoadMultipleServiceNetworks(t *testing.T) {
	networkCases := []struct {
		name            string
		networkBase     map[string]interface{}
		networkOverride map[string]interface{}
		expected        map[string]*types.ServiceNetworkConfig
	}{
		{
			name: "no_override",
			networkBase: map[string]interface{}{
				"networks": []interface{}{
					"net1",
					"net2",
				},
			},
			networkOverride: map[string]interface{}{},
			expected: map[string]*types.ServiceNetworkConfig{
				"net1": nil,
				"net2": nil,
			},
		},
		{
			name: "override_simple",
			networkBase: map[string]interface{}{
				"networks": []interface{}{
					"net1",
					"net2",
				},
			},
			networkOverride: map[string]interface{}{
				"networks": []interface{}{
					"net1",
					"net3",
				},
			},
			expected: map[string]*types.ServiceNetworkConfig{
				"net1": nil,
				"net2": nil,
				"net3": nil,
			},
		},
		{
			name: "override_with_aliases",
			networkBase: map[string]interface{}{
				"networks": map[string]interface{}{
					"net1": map[string]interface{}{
						"aliases": []interface{}{
							"alias1",
						},
					},
					"net2": nil,
				},
			},
			networkOverride: map[string]interface{}{
				"networks": map[string]interface{}{
					"net1": map[string]interface{}{
						"aliases": []interface{}{
							"alias2",
							"alias3",
						},
					},
					"net3": map[string]interface{}{},
				},
			},
			expected: map[string]*types.ServiceNetworkConfig{
				"net1": {
					Aliases: []string{"alias2", "alias3"},
				},
				"net2": nil,
				"net3": {},
			},
		},
	}

	for _, tc := range networkCases {
		t.Run(tc.name, func(t *testing.T) {
			configDetails := types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{
					{
						Filename: "base.yml",
						Config: map[string]interface{}{
							"version": "3.4",
							"services": map[string]interface{}{
								"foo": tc.networkBase,
							},
						},
					},
					{
						Filename: "override.yml",
						Config: map[string]interface{}{
							"version": "3.4",
							"services": map[string]interface{}{
								"foo": tc.networkOverride,
							},
						},
					},
				},
			}
			config, err := Load(configDetails)
			assert.NilError(t, err)
			assert.DeepEqual(t, &types.Config{
				Filename: "base.yml",
				Version:  "3.4",
				Services: []types.ServiceConfig{
					{
						Name:        "foo",
						Networks:    tc.expected,
						Environment: types.MappingWithEquals{},
					},
				},
				Networks: map[string]types.NetworkConfig{},
				Volumes:  map[string]types.VolumeConfig{},
				Secrets:  map[string]types.SecretConfig{},
				Configs:  map[string]types.ConfigObjConfig{},
			}, config)
		})
	}
}

func TestLoadMultipleConfigs(t *testing.T) {
	base := map[string]interface{}{
		"version": "3.4",
		"services": map[string]interface{}{
			"foo": map[string]interface{}{
				"image": "foo",
				"build": map[string]interface{}{
					"context":    ".",
					"dockerfile": "bar.Dockerfile",
				},
				"ports": []interface{}{
					"8080:80",
					"9090:90",
				},
				"labels": []interface{}{
					"foo=bar",
				},
				"cap_add": []interface{}{
					"NET_ADMIN",
				},
			},
		},
		"volumes":  map[string]interface{}{},
		"networks": map[string]interface{}{},
		"secrets":  map[string]interface{}{},
		"configs":  map[string]interface{}{},
	}
	override := map[string]interface{}{
		"version": "3.4",
		"services": map[string]interface{}{
			"foo": map[string]interface{}{
				"image": "baz",
				"build": map[string]interface{}{
					"dockerfile": "foo.Dockerfile",
					"args": []interface{}{
						"buildno=1",
						"password=secret",
					},
				},
				"ports": []interface{}{
					map[string]interface{}{
						"target":    81,
						"published": 8080,
					},
				},
				"labels": map[string]interface{}{
					"foo": "baz",
				},
				"cap_add": []interface{}{
					"SYS_ADMIN",
				},
			},
			"bar": map[string]interface{}{
				"image": "bar",
			},
		},
		"volumes":  map[string]interface{}{},
		"networks": map[string]interface{}{},
		"secrets":  map[string]interface{}{},
		"configs":  map[string]interface{}{},
	}
	configDetails := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "base.yml", Config: base},
			{Filename: "override.yml", Config: override},
		},
	}
	config, err := Load(configDetails)
	assert.NilError(t, err)
	assert.DeepEqual(t, &types.Config{
		Filename: "base.yml",
		Version:  "3.4",
		Services: []types.ServiceConfig{
			{
				Name:        "bar",
				Image:       "bar",
				Environment: types.MappingWithEquals{},
			},
			{
				Name:  "foo",
				Image: "baz",
				Build: types.BuildConfig{
					Context:    ".",
					Dockerfile: "foo.Dockerfile",
					Args: types.MappingWithEquals{
						"buildno":  strPtr("1"),
						"password": strPtr("secret"),
					},
				},
				Ports: []types.ServicePortConfig{
					{
						Target:    81,
						Published: 8080,
					},
					{
						Mode:      "ingress",
						Target:    90,
						Published: 9090,
						Protocol:  "tcp",
					},
				},
				Labels: types.Labels{
					"foo": "baz",
				},
				CapAdd:      []string{"NET_ADMIN", "SYS_ADMIN"},
				Environment: types.MappingWithEquals{},
			}},
		Networks: map[string]types.NetworkConfig{},
		Volumes:  map[string]types.VolumeConfig{},
		Secrets:  map[string]types.SecretConfig{},
		Configs:  map[string]types.ConfigObjConfig{},
	}, config)
}

// Issue#972
func TestLoadMultipleNetworks(t *testing.T) {
	base := map[string]interface{}{
		"version": "3.4",
		"services": map[string]interface{}{
			"foo": map[string]interface{}{
				"image": "baz",
			},
		},
		"volumes": map[string]interface{}{},
		"networks": map[string]interface{}{
			"hostnet": map[string]interface{}{
				"driver": "overlay",
				"ipam": map[string]interface{}{
					"driver": "default",
					"config": []interface{}{
						map[string]interface{}{
							"subnet": "10.0.0.0/20",
						},
					},
				},
			},
		},
		"secrets": map[string]interface{}{},
		"configs": map[string]interface{}{},
	}
	override := map[string]interface{}{
		"version":  "3.4",
		"services": map[string]interface{}{},
		"volumes":  map[string]interface{}{},
		"networks": map[string]interface{}{
			"hostnet": map[string]interface{}{
				"external": map[string]interface{}{
					"name": "host",
				},
			},
		},
		"secrets": map[string]interface{}{},
		"configs": map[string]interface{}{},
	}
	configDetails := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "base.yml", Config: base},
			{Filename: "override.yml", Config: override},
		},
	}
	config, err := Load(configDetails)
	assert.NilError(t, err)
	assert.DeepEqual(t, &types.Config{
		Filename: "base.yml",
		Version:  "3.4",
		Services: []types.ServiceConfig{
			{
				Name:        "foo",
				Image:       "baz",
				Environment: types.MappingWithEquals{},
			}},
		Networks: map[string]types.NetworkConfig{
			"hostnet": {
				Name: "host",
				External: types.External{
					External: true,
				},
			},
		},
		Volumes: map[string]types.VolumeConfig{},
		Secrets: map[string]types.SecretConfig{},
		Configs: map[string]types.ConfigObjConfig{},
	}, config)
}
