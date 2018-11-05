package convert

import (
	"testing"

	composetypes "github.com/docker/cli/cli/compose/types"
	"github.com/docker/docker/api/types/mount"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestConvertVolumeToMountAnonymousVolume(t *testing.T) {
	config := composetypes.ServiceVolumeConfig{
		Type:   "volume",
		Target: "/foo/bar",
	}
	expected := mount.Mount{
		Type:   mount.TypeVolume,
		Target: "/foo/bar",
	}
	mount, err := convertVolumeToMount(config, volumes{}, NewNamespace("foo"))
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, mount))
}

func TestConvertVolumeToMountAnonymousBind(t *testing.T) {
	config := composetypes.ServiceVolumeConfig{
		Type:   "bind",
		Target: "/foo/bar",
		Bind: &composetypes.ServiceVolumeBind{
			Propagation: "slave",
		},
	}
	_, err := convertVolumeToMount(config, volumes{}, NewNamespace("foo"))
	assert.Error(t, err, "invalid bind source, source cannot be empty")
}

func TestConvertVolumeToMountUnapprovedType(t *testing.T) {
	config := composetypes.ServiceVolumeConfig{
		Type:   "foo",
		Target: "/foo/bar",
	}
	_, err := convertVolumeToMount(config, volumes{}, NewNamespace("foo"))
	assert.Error(t, err, "volume type must be volume, bind, tmpfs or npipe")
}

func TestConvertVolumeToMountConflictingOptionsBindInVolume(t *testing.T) {
	namespace := NewNamespace("foo")

	config := composetypes.ServiceVolumeConfig{
		Type:   "volume",
		Source: "foo",
		Target: "/target",
		Bind: &composetypes.ServiceVolumeBind{
			Propagation: "slave",
		},
	}
	_, err := convertVolumeToMount(config, volumes{}, namespace)
	assert.Error(t, err, "bind options are incompatible with type volume")
}

func TestConvertVolumeToMountConflictingOptionsTmpfsInVolume(t *testing.T) {
	namespace := NewNamespace("foo")

	config := composetypes.ServiceVolumeConfig{
		Type:   "volume",
		Source: "foo",
		Target: "/target",
		Tmpfs: &composetypes.ServiceVolumeTmpfs{
			Size: 1000,
		},
	}
	_, err := convertVolumeToMount(config, volumes{}, namespace)
	assert.Error(t, err, "tmpfs options are incompatible with type volume")
}

func TestConvertVolumeToMountConflictingOptionsVolumeInBind(t *testing.T) {
	namespace := NewNamespace("foo")

	config := composetypes.ServiceVolumeConfig{
		Type:   "bind",
		Source: "/foo",
		Target: "/target",
		Volume: &composetypes.ServiceVolumeVolume{
			NoCopy: true,
		},
	}
	_, err := convertVolumeToMount(config, volumes{}, namespace)
	assert.Error(t, err, "volume options are incompatible with type bind")
}

func TestConvertVolumeToMountConflictingOptionsTmpfsInBind(t *testing.T) {
	namespace := NewNamespace("foo")

	config := composetypes.ServiceVolumeConfig{
		Type:   "bind",
		Source: "/foo",
		Target: "/target",
		Tmpfs: &composetypes.ServiceVolumeTmpfs{
			Size: 1000,
		},
	}
	_, err := convertVolumeToMount(config, volumes{}, namespace)
	assert.Error(t, err, "tmpfs options are incompatible with type bind")
}

func TestConvertVolumeToMountConflictingOptionsBindInTmpfs(t *testing.T) {
	namespace := NewNamespace("foo")

	config := composetypes.ServiceVolumeConfig{
		Type:   "tmpfs",
		Target: "/target",
		Bind: &composetypes.ServiceVolumeBind{
			Propagation: "slave",
		},
	}
	_, err := convertVolumeToMount(config, volumes{}, namespace)
	assert.Error(t, err, "bind options are incompatible with type tmpfs")
}

func TestConvertVolumeToMountConflictingOptionsVolumeInTmpfs(t *testing.T) {
	namespace := NewNamespace("foo")

	config := composetypes.ServiceVolumeConfig{
		Type:   "tmpfs",
		Target: "/target",
		Volume: &composetypes.ServiceVolumeVolume{
			NoCopy: true,
		},
	}
	_, err := convertVolumeToMount(config, volumes{}, namespace)
	assert.Error(t, err, "volume options are incompatible with type tmpfs")
}

func TestConvertVolumeToMountNamedVolume(t *testing.T) {
	stackVolumes := volumes{
		"normal": composetypes.VolumeConfig{
			Driver: "glusterfs",
			DriverOpts: map[string]string{
				"opt": "value",
			},
			Labels: map[string]string{
				"something": "labeled",
			},
		},
	}
	namespace := NewNamespace("foo")
	expected := mount.Mount{
		Type:     mount.TypeVolume,
		Source:   "foo_normal",
		Target:   "/foo",
		ReadOnly: true,
		VolumeOptions: &mount.VolumeOptions{
			Labels: map[string]string{
				LabelNamespace: "foo",
				"something":    "labeled",
			},
			DriverConfig: &mount.Driver{
				Name: "glusterfs",
				Options: map[string]string{
					"opt": "value",
				},
			},
			NoCopy: true,
		},
	}
	config := composetypes.ServiceVolumeConfig{
		Type:     "volume",
		Source:   "normal",
		Target:   "/foo",
		ReadOnly: true,
		Volume: &composetypes.ServiceVolumeVolume{
			NoCopy: true,
		},
	}
	mount, err := convertVolumeToMount(config, stackVolumes, namespace)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, mount))
}

func TestConvertVolumeToMountNamedVolumeWithNameCustomizd(t *testing.T) {
	stackVolumes := volumes{
		"normal": composetypes.VolumeConfig{
			Name:   "user_specified_name",
			Driver: "vsphere",
			DriverOpts: map[string]string{
				"opt": "value",
			},
			Labels: map[string]string{
				"something": "labeled",
			},
		},
	}
	namespace := NewNamespace("foo")
	expected := mount.Mount{
		Type:     mount.TypeVolume,
		Source:   "user_specified_name",
		Target:   "/foo",
		ReadOnly: true,
		VolumeOptions: &mount.VolumeOptions{
			Labels: map[string]string{
				LabelNamespace: "foo",
				"something":    "labeled",
			},
			DriverConfig: &mount.Driver{
				Name: "vsphere",
				Options: map[string]string{
					"opt": "value",
				},
			},
			NoCopy: true,
		},
	}
	config := composetypes.ServiceVolumeConfig{
		Type:     "volume",
		Source:   "normal",
		Target:   "/foo",
		ReadOnly: true,
		Volume: &composetypes.ServiceVolumeVolume{
			NoCopy: true,
		},
	}
	mount, err := convertVolumeToMount(config, stackVolumes, namespace)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, mount))
}

func TestConvertVolumeToMountNamedVolumeExternal(t *testing.T) {
	stackVolumes := volumes{
		"outside": composetypes.VolumeConfig{
			Name:     "special",
			External: composetypes.External{External: true},
		},
	}
	namespace := NewNamespace("foo")
	expected := mount.Mount{
		Type:          mount.TypeVolume,
		Source:        "special",
		Target:        "/foo",
		VolumeOptions: &mount.VolumeOptions{NoCopy: false},
	}
	config := composetypes.ServiceVolumeConfig{
		Type:   "volume",
		Source: "outside",
		Target: "/foo",
	}
	mount, err := convertVolumeToMount(config, stackVolumes, namespace)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, mount))
}

func TestConvertVolumeToMountNamedVolumeExternalNoCopy(t *testing.T) {
	stackVolumes := volumes{
		"outside": composetypes.VolumeConfig{
			Name:     "special",
			External: composetypes.External{External: true},
		},
	}
	namespace := NewNamespace("foo")
	expected := mount.Mount{
		Type:   mount.TypeVolume,
		Source: "special",
		Target: "/foo",
		VolumeOptions: &mount.VolumeOptions{
			NoCopy: true,
		},
	}
	config := composetypes.ServiceVolumeConfig{
		Type:   "volume",
		Source: "outside",
		Target: "/foo",
		Volume: &composetypes.ServiceVolumeVolume{
			NoCopy: true,
		},
	}
	mount, err := convertVolumeToMount(config, stackVolumes, namespace)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, mount))
}

func TestConvertVolumeToMountBind(t *testing.T) {
	stackVolumes := volumes{}
	namespace := NewNamespace("foo")
	expected := mount.Mount{
		Type:        mount.TypeBind,
		Source:      "/bar",
		Target:      "/foo",
		ReadOnly:    true,
		BindOptions: &mount.BindOptions{Propagation: mount.PropagationShared},
	}
	config := composetypes.ServiceVolumeConfig{
		Type:     "bind",
		Source:   "/bar",
		Target:   "/foo",
		ReadOnly: true,
		Bind:     &composetypes.ServiceVolumeBind{Propagation: "shared"},
	}
	mount, err := convertVolumeToMount(config, stackVolumes, namespace)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, mount))
}

func TestConvertVolumeToMountVolumeDoesNotExist(t *testing.T) {
	namespace := NewNamespace("foo")
	config := composetypes.ServiceVolumeConfig{
		Type:     "volume",
		Source:   "unknown",
		Target:   "/foo",
		ReadOnly: true,
	}
	_, err := convertVolumeToMount(config, volumes{}, namespace)
	assert.Error(t, err, "undefined volume \"unknown\"")
}

func TestConvertTmpfsToMountVolume(t *testing.T) {
	config := composetypes.ServiceVolumeConfig{
		Type:   "tmpfs",
		Target: "/foo/bar",
		Tmpfs: &composetypes.ServiceVolumeTmpfs{
			Size: 1000,
		},
	}
	expected := mount.Mount{
		Type:         mount.TypeTmpfs,
		Target:       "/foo/bar",
		TmpfsOptions: &mount.TmpfsOptions{SizeBytes: 1000},
	}
	mount, err := convertVolumeToMount(config, volumes{}, NewNamespace("foo"))
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, mount))
}

func TestConvertTmpfsToMountVolumeWithSource(t *testing.T) {
	config := composetypes.ServiceVolumeConfig{
		Type:   "tmpfs",
		Source: "/bar",
		Target: "/foo/bar",
		Tmpfs: &composetypes.ServiceVolumeTmpfs{
			Size: 1000,
		},
	}

	_, err := convertVolumeToMount(config, volumes{}, NewNamespace("foo"))
	assert.Error(t, err, "invalid tmpfs source, source must be empty")
}

func TestConvertVolumeToMountAnonymousNpipe(t *testing.T) {
	config := composetypes.ServiceVolumeConfig{
		Type:   "npipe",
		Source: `\\.\pipe\foo`,
		Target: `\\.\pipe\foo`,
	}
	expected := mount.Mount{
		Type:   mount.TypeNamedPipe,
		Source: `\\.\pipe\foo`,
		Target: `\\.\pipe\foo`,
	}
	mount, err := convertVolumeToMount(config, volumes{}, NewNamespace("foo"))
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, mount))
}
