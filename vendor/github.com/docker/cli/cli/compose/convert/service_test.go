package convert

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	composetypes "github.com/docker/cli/cli/compose/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestConvertRestartPolicyFromNone(t *testing.T) {
	policy, err := convertRestartPolicy("no", nil)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual((*swarm.RestartPolicy)(nil), policy))
}

func TestConvertRestartPolicyFromUnknown(t *testing.T) {
	_, err := convertRestartPolicy("unknown", nil)
	assert.Error(t, err, "unknown restart policy: unknown")
}

func TestConvertRestartPolicyFromAlways(t *testing.T) {
	policy, err := convertRestartPolicy("always", nil)
	expected := &swarm.RestartPolicy{
		Condition: swarm.RestartPolicyConditionAny,
	}
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, policy))
}

func TestConvertRestartPolicyFromFailure(t *testing.T) {
	policy, err := convertRestartPolicy("on-failure:4", nil)
	attempts := uint64(4)
	expected := &swarm.RestartPolicy{
		Condition:   swarm.RestartPolicyConditionOnFailure,
		MaxAttempts: &attempts,
	}
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, policy))
}

func strPtr(val string) *string {
	return &val
}

func TestConvertEnvironment(t *testing.T) {
	source := map[string]*string{
		"foo": strPtr("bar"),
		"key": strPtr("value"),
	}
	env := convertEnvironment(source)
	sort.Strings(env)
	assert.Check(t, is.DeepEqual([]string{"foo=bar", "key=value"}, env))
}

func TestConvertExtraHosts(t *testing.T) {
	source := composetypes.HostsList{
		"zulu:127.0.0.2",
		"alpha:127.0.0.1",
		"zulu:ff02::1",
	}
	assert.Check(t, is.DeepEqual([]string{"127.0.0.2 zulu", "127.0.0.1 alpha", "ff02::1 zulu"}, convertExtraHosts(source)))
}

func TestConvertResourcesFull(t *testing.T) {
	source := composetypes.Resources{
		Limits: &composetypes.Resource{
			NanoCPUs:    "0.003",
			MemoryBytes: composetypes.UnitBytes(300000000),
		},
		Reservations: &composetypes.Resource{
			NanoCPUs:    "0.002",
			MemoryBytes: composetypes.UnitBytes(200000000),
		},
	}
	resources, err := convertResources(source)
	assert.NilError(t, err)

	expected := &swarm.ResourceRequirements{
		Limits: &swarm.Resources{
			NanoCPUs:    3000000,
			MemoryBytes: 300000000,
		},
		Reservations: &swarm.Resources{
			NanoCPUs:    2000000,
			MemoryBytes: 200000000,
		},
	}
	assert.Check(t, is.DeepEqual(expected, resources))
}

func TestConvertResourcesOnlyMemory(t *testing.T) {
	source := composetypes.Resources{
		Limits: &composetypes.Resource{
			MemoryBytes: composetypes.UnitBytes(300000000),
		},
		Reservations: &composetypes.Resource{
			MemoryBytes: composetypes.UnitBytes(200000000),
		},
	}
	resources, err := convertResources(source)
	assert.NilError(t, err)

	expected := &swarm.ResourceRequirements{
		Limits: &swarm.Resources{
			MemoryBytes: 300000000,
		},
		Reservations: &swarm.Resources{
			MemoryBytes: 200000000,
		},
	}
	assert.Check(t, is.DeepEqual(expected, resources))
}

func TestConvertHealthcheck(t *testing.T) {
	retries := uint64(10)
	timeout := composetypes.Duration(30 * time.Second)
	interval := composetypes.Duration(2 * time.Millisecond)
	source := &composetypes.HealthCheckConfig{
		Test:     []string{"EXEC", "touch", "/foo"},
		Timeout:  &timeout,
		Interval: &interval,
		Retries:  &retries,
	}
	expected := &container.HealthConfig{
		Test:     source.Test,
		Timeout:  time.Duration(timeout),
		Interval: time.Duration(interval),
		Retries:  10,
	}

	healthcheck, err := convertHealthcheck(source)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, healthcheck))
}

func TestConvertHealthcheckDisable(t *testing.T) {
	source := &composetypes.HealthCheckConfig{Disable: true}
	expected := &container.HealthConfig{
		Test: []string{"NONE"},
	}

	healthcheck, err := convertHealthcheck(source)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, healthcheck))
}

func TestConvertHealthcheckDisableWithTest(t *testing.T) {
	source := &composetypes.HealthCheckConfig{
		Disable: true,
		Test:    []string{"EXEC", "touch"},
	}
	_, err := convertHealthcheck(source)
	assert.Error(t, err, "test and disable can't be set at the same time")
}

func TestConvertEndpointSpec(t *testing.T) {
	source := []composetypes.ServicePortConfig{
		{
			Protocol:  "udp",
			Target:    53,
			Published: 1053,
			Mode:      "host",
		},
		{
			Target:    8080,
			Published: 80,
		},
	}
	endpoint, err := convertEndpointSpec("vip", source)

	expected := swarm.EndpointSpec{
		Mode: swarm.ResolutionMode(strings.ToLower("vip")),
		Ports: []swarm.PortConfig{
			{
				TargetPort:    8080,
				PublishedPort: 80,
			},
			{
				Protocol:      "udp",
				TargetPort:    53,
				PublishedPort: 1053,
				PublishMode:   "host",
			},
		},
	}

	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, *endpoint))
}

func TestConvertServiceNetworksOnlyDefault(t *testing.T) {
	networkConfigs := networkMap{}

	configs, err := convertServiceNetworks(
		nil, networkConfigs, NewNamespace("foo"), "service")

	expected := []swarm.NetworkAttachmentConfig{
		{
			Target:  "foo_default",
			Aliases: []string{"service"},
		},
	}

	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, configs))
}

func TestConvertServiceNetworks(t *testing.T) {
	networkConfigs := networkMap{
		"front": composetypes.NetworkConfig{
			External: composetypes.External{External: true},
			Name:     "fronttier",
		},
		"back": composetypes.NetworkConfig{},
	}
	networks := map[string]*composetypes.ServiceNetworkConfig{
		"front": {
			Aliases: []string{"something"},
		},
		"back": {
			Aliases: []string{"other"},
		},
	}

	configs, err := convertServiceNetworks(
		networks, networkConfigs, NewNamespace("foo"), "service")

	expected := []swarm.NetworkAttachmentConfig{
		{
			Target:  "foo_back",
			Aliases: []string{"other", "service"},
		},
		{
			Target:  "fronttier",
			Aliases: []string{"something", "service"},
		},
	}

	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, configs))
}

func TestConvertServiceNetworksCustomDefault(t *testing.T) {
	networkConfigs := networkMap{
		"default": composetypes.NetworkConfig{
			External: composetypes.External{External: true},
			Name:     "custom",
		},
	}
	networks := map[string]*composetypes.ServiceNetworkConfig{}

	configs, err := convertServiceNetworks(
		networks, networkConfigs, NewNamespace("foo"), "service")

	expected := []swarm.NetworkAttachmentConfig{
		{
			Target:  "custom",
			Aliases: []string{"service"},
		},
	}

	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, []swarm.NetworkAttachmentConfig(configs)))
}

func TestConvertDNSConfigEmpty(t *testing.T) {
	dnsConfig, err := convertDNSConfig(nil, nil)

	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual((*swarm.DNSConfig)(nil), dnsConfig))
}

var (
	nameservers = []string{"8.8.8.8", "9.9.9.9"}
	search      = []string{"dc1.example.com", "dc2.example.com"}
)

func TestConvertDNSConfigAll(t *testing.T) {
	dnsConfig, err := convertDNSConfig(nameservers, search)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(&swarm.DNSConfig{
		Nameservers: nameservers,
		Search:      search,
	}, dnsConfig))
}

func TestConvertDNSConfigNameservers(t *testing.T) {
	dnsConfig, err := convertDNSConfig(nameservers, nil)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(&swarm.DNSConfig{
		Nameservers: nameservers,
		Search:      nil,
	}, dnsConfig))
}

func TestConvertDNSConfigSearch(t *testing.T) {
	dnsConfig, err := convertDNSConfig(nil, search)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(&swarm.DNSConfig{
		Nameservers: nil,
		Search:      search,
	}, dnsConfig))
}

func TestConvertCredentialSpec(t *testing.T) {
	swarmSpec, err := convertCredentialSpec(composetypes.CredentialSpecConfig{})
	assert.NilError(t, err)
	assert.Check(t, is.Nil(swarmSpec))

	swarmSpec, err = convertCredentialSpec(composetypes.CredentialSpecConfig{
		File: "/foo",
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(swarmSpec.File, "/foo"))
	assert.Check(t, is.Equal(swarmSpec.Registry, ""))

	swarmSpec, err = convertCredentialSpec(composetypes.CredentialSpecConfig{
		Registry: "foo",
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(swarmSpec.File, ""))
	assert.Check(t, is.Equal(swarmSpec.Registry, "foo"))

	swarmSpec, err = convertCredentialSpec(composetypes.CredentialSpecConfig{
		File:     "/asdf",
		Registry: "foo",
	})
	assert.Check(t, is.ErrorContains(err, ""))
	assert.Check(t, is.Nil(swarmSpec))
}

func TestConvertUpdateConfigOrder(t *testing.T) {
	// test default behavior
	updateConfig := convertUpdateConfig(&composetypes.UpdateConfig{})
	assert.Check(t, is.Equal("", updateConfig.Order))

	// test start-first
	updateConfig = convertUpdateConfig(&composetypes.UpdateConfig{
		Order: "start-first",
	})
	assert.Check(t, is.Equal(updateConfig.Order, "start-first"))

	// test stop-first
	updateConfig = convertUpdateConfig(&composetypes.UpdateConfig{
		Order: "stop-first",
	})
	assert.Check(t, is.Equal(updateConfig.Order, "stop-first"))
}

func TestConvertFileObject(t *testing.T) {
	namespace := NewNamespace("testing")
	config := composetypes.FileReferenceConfig{
		Source: "source",
		Target: "target",
		UID:    "user",
		GID:    "group",
		Mode:   uint32Ptr(0644),
	}
	swarmRef, err := convertFileObject(namespace, config, lookupConfig)
	assert.NilError(t, err)

	expected := swarmReferenceObject{
		Name: "testing_source",
		File: swarmReferenceTarget{
			Name: config.Target,
			UID:  config.UID,
			GID:  config.GID,
			Mode: os.FileMode(0644),
		},
	}
	assert.Check(t, is.DeepEqual(expected, swarmRef))
}

func lookupConfig(key string) (composetypes.FileObjectConfig, error) {
	if key != "source" {
		return composetypes.FileObjectConfig{}, errors.New("bad key")
	}
	return composetypes.FileObjectConfig{}, nil
}

func TestConvertFileObjectDefaults(t *testing.T) {
	namespace := NewNamespace("testing")
	config := composetypes.FileReferenceConfig{Source: "source"}
	swarmRef, err := convertFileObject(namespace, config, lookupConfig)
	assert.NilError(t, err)

	expected := swarmReferenceObject{
		Name: "testing_source",
		File: swarmReferenceTarget{
			Name: config.Source,
			UID:  "0",
			GID:  "0",
			Mode: os.FileMode(0444),
		},
	}
	assert.Check(t, is.DeepEqual(expected, swarmRef))
}

func TestServiceConvertsIsolation(t *testing.T) {
	src := composetypes.ServiceConfig{
		Isolation: "hyperv",
	}
	result, err := Service("1.35", Namespace{name: "foo"}, src, nil, nil, nil, nil)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(container.IsolationHyperV, result.TaskTemplate.ContainerSpec.Isolation))
}

func TestConvertServiceSecrets(t *testing.T) {
	namespace := Namespace{name: "foo"}
	secrets := []composetypes.ServiceSecretConfig{
		{Source: "foo_secret"},
		{Source: "bar_secret"},
	}
	secretSpecs := map[string]composetypes.SecretConfig{
		"foo_secret": {
			Name: "foo_secret",
		},
		"bar_secret": {
			Name: "bar_secret",
		},
	}
	client := &fakeClient{
		secretListFunc: func(opts types.SecretListOptions) ([]swarm.Secret, error) {
			assert.Check(t, is.Contains(opts.Filters.Get("name"), "foo_secret"))
			assert.Check(t, is.Contains(opts.Filters.Get("name"), "bar_secret"))
			return []swarm.Secret{
				{Spec: swarm.SecretSpec{Annotations: swarm.Annotations{Name: "foo_secret"}}},
				{Spec: swarm.SecretSpec{Annotations: swarm.Annotations{Name: "bar_secret"}}},
			}, nil
		},
	}
	refs, err := convertServiceSecrets(client, namespace, secrets, secretSpecs)
	assert.NilError(t, err)
	expected := []*swarm.SecretReference{
		{
			SecretName: "bar_secret",
			File: &swarm.SecretReferenceFileTarget{
				Name: "bar_secret",
				UID:  "0",
				GID:  "0",
				Mode: 0444,
			},
		},
		{
			SecretName: "foo_secret",
			File: &swarm.SecretReferenceFileTarget{
				Name: "foo_secret",
				UID:  "0",
				GID:  "0",
				Mode: 0444,
			},
		},
	}
	assert.DeepEqual(t, expected, refs)
}

func TestConvertServiceConfigs(t *testing.T) {
	namespace := Namespace{name: "foo"}
	configs := []composetypes.ServiceConfigObjConfig{
		{Source: "foo_config"},
		{Source: "bar_config"},
	}
	configSpecs := map[string]composetypes.ConfigObjConfig{
		"foo_config": {
			Name: "foo_config",
		},
		"bar_config": {
			Name: "bar_config",
		},
	}
	client := &fakeClient{
		configListFunc: func(opts types.ConfigListOptions) ([]swarm.Config, error) {
			assert.Check(t, is.Contains(opts.Filters.Get("name"), "foo_config"))
			assert.Check(t, is.Contains(opts.Filters.Get("name"), "bar_config"))
			return []swarm.Config{
				{Spec: swarm.ConfigSpec{Annotations: swarm.Annotations{Name: "foo_config"}}},
				{Spec: swarm.ConfigSpec{Annotations: swarm.Annotations{Name: "bar_config"}}},
			}, nil
		},
	}
	refs, err := convertServiceConfigObjs(client, namespace, configs, configSpecs)
	assert.NilError(t, err)
	expected := []*swarm.ConfigReference{
		{
			ConfigName: "bar_config",
			File: &swarm.ConfigReferenceFileTarget{
				Name: "bar_config",
				UID:  "0",
				GID:  "0",
				Mode: 0444,
			},
		},
		{
			ConfigName: "foo_config",
			File: &swarm.ConfigReferenceFileTarget{
				Name: "foo_config",
				UID:  "0",
				GID:  "0",
				Mode: 0444,
			},
		},
	}
	assert.DeepEqual(t, expected, refs)
}

type fakeClient struct {
	client.Client
	secretListFunc func(types.SecretListOptions) ([]swarm.Secret, error)
	configListFunc func(types.ConfigListOptions) ([]swarm.Config, error)
}

func (c *fakeClient) SecretList(ctx context.Context, options types.SecretListOptions) ([]swarm.Secret, error) {
	if c.secretListFunc != nil {
		return c.secretListFunc(options)
	}
	return []swarm.Secret{}, nil
}

func (c *fakeClient) ConfigList(ctx context.Context, options types.ConfigListOptions) ([]swarm.Config, error) {
	if c.configListFunc != nil {
		return c.configListFunc(options)
	}
	return []swarm.Config{}, nil
}

func TestConvertUpdateConfigParallelism(t *testing.T) {
	parallel := uint64(4)

	// test default behavior
	updateConfig := convertUpdateConfig(&composetypes.UpdateConfig{})
	assert.Check(t, is.Equal(uint64(1), updateConfig.Parallelism))

	// Non default value
	updateConfig = convertUpdateConfig(&composetypes.UpdateConfig{
		Parallelism: &parallel,
	})
	assert.Check(t, is.Equal(parallel, updateConfig.Parallelism))
}
