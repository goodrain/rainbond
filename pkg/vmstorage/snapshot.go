// Package vmstorage computes advisory VM storage capabilities from live cluster evidence.
// It never creates resources or assigns logical-to-physical disk bindings.
package vmstorage

import (
	"context"
	"path"
	"reflect"
	"sort"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	// ManagedSnapshotDriver is retained for the original qualified CephFS path.
	ManagedSnapshotDriver = "cephfs.csi.ceph.com"
	// LonghornSnapshotDriver is the qualified lightweight CSI snapshot path.
	LonghornSnapshotDriver = "driver.longhorn.io"
)

func managedSnapshotDriver(driver string) bool {
	return driver == ManagedSnapshotDriver || driver == LonghornSnapshotDriver
}

// ManagedSnapshotCapability applies the current managed driver policy to native
// CSI evidence. It is storage eligibility, not permission to run a VM operation.
func ManagedSnapshotCapability(native Capability) Capability {
	managed := native
	if managed.Status == "unknown" {
		managed.Supported = false
		return managed
	}
	if managed.Driver == "" {
		managed.Supported, managed.Status = false, "unknown"
		managed.ReasonCode = "managed_snapshot_driver_unverified"
		managed.Message = "无法核验当前存储的实际 CSI 驱动"
		return managed
	}
	if !managedSnapshotDriver(managed.Driver) {
		managed.Supported, managed.Status = false, "unsupported"
		managed.ReasonCode = "managed_snapshot_driver_unsupported"
		managed.Message = "该存储的 CSI 驱动不在受管虚拟机快照支持范围"
	} else if managed.Supported {
		switch {
		case managed.backingSnapshot:
			managed.Supported, managed.Status = false, "unsupported"
			managed.ReasonCode, managed.Message = "managed_snapshot_backing_unsupported", "受管快照不支持backingSnapshot存储模式"
		case managed.deletionPolicy == "":
			managed.Supported, managed.Status = false, "unknown"
			managed.ReasonCode, managed.Message = "managed_snapshot_class_policy_unverified", "无法核验所选快照类的删除策略"
		case managed.deletionPolicy != "Delete":
			managed.Supported, managed.Status = false, "unsupported"
			managed.ReasonCode, managed.Message = "managed_snapshot_class_policy_unsupported", "受管快照要求所选快照类使用Delete策略"
		default:
			managed.Message = "该存储驱动适用于受管快照，实际虚拟机和任务条件仍需核验"
		}
	}
	return managed
}

var (
	snapshotClassGVR  = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotclasses"}
	volumeSnapshotGVR = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	vmSnapshotGVR     = schema.GroupVersionResource{Group: "snapshot.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinesnapshots"}
	kubeVirtGVR       = schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "kubevirts"}
)

// Capability is fail-closed: supported is true only when status is supported.
// Unknown denotes missing/read-failed readiness evidence, not an unsupported driver.
type Capability struct {
	Supported           bool   `json:"supported"`
	Status              string `json:"status"`
	ReasonCode          string `json:"reason_code"`
	Message             string `json:"message"`
	StorageClass        string `json:"storage_class,omitempty"`
	Driver              string `json:"driver,omitempty"`
	VolumeSnapshotClass string `json:"volume_snapshot_class,omitempty"`
	// Private live selection evidence is never accepted from an API caller.
	deletionPolicy  string
	backingSnapshot bool
}

// Volume identifies one participating disk. It does not assign a backing PVC.
type Volume struct {
	Key          string
	StorageClass string
}

// VolumeCapability preserves every participating disk's result.
type VolumeCapability struct {
	Key string `json:"key"`
	Capability
}

// SnapshotSet requires the complete disk set; unsupported/unknown disks are never excluded.
type SnapshotSet struct {
	Capability
	Volumes []VolumeCapability `json:"volumes"`
}

// Inspector is request-scoped. Its live service/class snapshot is shared only within one query.
// Snapshot creators must construct a fresh inspector immediately before submission.
type Inspector struct {
	core    kubernetes.Interface
	dynamic dynamic.Interface
	once    sync.Once
	classes []unstructured.Unstructured
	pods    []corev1.Pod
	global  *Capability
}

// NewInspector uses existing Kubernetes clients without cached database class details.
func NewInspector(core kubernetes.Interface, dyn dynamic.Interface) *Inspector {
	if nilClient(core) {
		core = nil
	}
	if nilClient(dyn) {
		dyn = nil
	}
	return &Inspector{core: core, dynamic: dyn}
}
func decision(status, code, message string) Capability {
	return Capability{Supported: status == "supported", Status: status, ReasonCode: code, Message: message}
}
func unknown(code, message string) Capability { return decision("unknown", code, message) }

func (i *Inspector) load(ctx context.Context) {
	fail := func(code, message string) { c := unknown(code, message); i.global = &c }
	classes, err := i.dynamic.Resource(snapshotClassGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		fail("snapshot_class_read_failed", "无法读取v1快照类，不能确认快照能力")
		return
	}
	i.classes = classes.Items
	for _, gvr := range []schema.GroupVersionResource{volumeSnapshotGVR, vmSnapshotGVR} {
		if _, err = i.dynamic.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
			fail("snapshot_api_read_failed", "快照API不可读取或未就绪")
			return
		}
	}
	kvs, err := i.dynamic.Resource(kubeVirtGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		fail("kubevirt_read_failed", "无法读取虚拟机服务就绪状态")
		return
	}
	if len(kvs.Items) != 1 {
		fail("kubevirt_not_ready", "虚拟机服务未能唯一确认就绪")
		return
	}
	kv := kvs.Items[0]
	phase, _, _ := unstructured.NestedString(kv.Object, "status", "phase")
	conditions, _, _ := unstructured.NestedSlice(kv.Object, "status", "conditions")
	available, degraded := false, false
	for _, c := range conditions {
		m, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if m["type"] == "Available" && m["status"] == "True" {
			available = true
		}
		if m["type"] == "Degraded" && m["status"] == "True" {
			degraded = true
		}
	}
	if phase != "Deployed" || !available || degraded || kv.GetDeletionTimestamp() != nil {
		fail("kubevirt_not_ready", "虚拟机服务未就绪")
		return
	}
	pods, err := i.core.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		fail("snapshot_services_read_failed", "无法读取快照服务就绪状态")
		return
	}
	i.pods = pods.Items
	for n := range i.pods {
		p := &i.pods[n]
		if !podReady(p) {
			continue
		}
		for _, c := range p.Spec.Containers {
			if component(c, "snapshot-controller") && containerReady(p, c.Name) {
				return
			}
		}
	}
	fail("snapshot_controller_not_ready", "快照控制器未就绪或缺少可核验的运行证据")
}

// StorageClass evaluates actual provisioner identity, matching snapshot class and service readiness.
func (i *Inspector) StorageClass(ctx context.Context, name string) Capability {
	if i == nil || i.core == nil || i.dynamic == nil {
		return unknown("capability_client_unavailable", "存储能力查询客户端不可用")
	}
	if strings.TrimSpace(name) == "" {
		return unknown("storage_class_missing", "磁盘未指定可核验的存储类")
	}
	sc, err := i.core.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return unknown("storage_class_read_failed", "无法读取当前存储类配置")
	}
	finish := func(c Capability) Capability { c.StorageClass = name; c.Driver = sc.Provisioner; return c }
	if sc.DeletionTimestamp != nil {
		return finish(unknown("storage_class_not_ready", "存储类正在删除"))
	}
	if _, err = i.core.StorageV1().CSIDrivers().Get(ctx, sc.Provisioner, metav1.GetOptions{}); apierrors.IsNotFound(err) {
		return finish(decision("unsupported", "csi_driver_not_registered", "该存储提供方没有注册CSI驱动，不支持此VM快照路径"))
	} else if err != nil {
		return finish(unknown("csi_driver_read_failed", "无法核验CSI驱动注册状态"))
	}
	i.once.Do(func() { i.load(ctx) })
	if i.global != nil {
		return finish(*i.global)
	}
	base := SelectSnapshotClass(ctx, i.dynamic, sc, i.classes)
	if !base.Supported {
		return base
	}
	driverSockets, snapshotterSockets := map[string]bool{}, map[string]bool{}
	ready := func() Capability {
		base.Message = "驱动、快照类与服务已就绪；实际快照结果以任务终态为准"
		return base
	}
	for n := range i.pods {
		p := &i.pods[n]
		if !podReady(p) {
			continue
		}
		driverReady, snapshotterReady := false, false
		for _, c := range p.Spec.Containers {
			if driverName(c) == sc.Provisioner && containerReady(p, c.Name) {
				driverReady = true
				if p.Spec.NodeName != "" {
					for _, socket := range csiSocketHostPaths(p, c) {
						driverSockets[p.Spec.NodeName+"\x00"+socket] = true
					}
				}
			}
			if component(c, "csi-snapshotter") && containerReady(p, c.Name) {
				snapshotterReady = true
				if p.Spec.NodeName != "" {
					for _, socket := range csiSocketHostPaths(p, c) {
						snapshotterSockets[p.Spec.NodeName+"\x00"+socket] = true
					}
				}
			}
		}
		if driverReady && snapshotterReady {
			return ready()
		}
	}
	for socket := range driverSockets {
		if snapshotterSockets[socket] {
			return ready()
		}
	}
	c := finish(unknown("driver_snapshotter_not_ready", "该CSI驱动的快照sidecar未就绪或缺少可核验的驱动关联证据"))
	c.VolumeSnapshotClass = base.VolumeSnapshotClass
	return c
}

// SelectSnapshotClass shares the exact StorageProfile/default selection and
// managed policy evidence with creation. A supported result only proves class
// selection; Inspector must additionally verify CSI and controller readiness.
func SelectSnapshotClass(ctx context.Context, dyn dynamic.Interface, sc *storagev1.StorageClass, classes []unstructured.Unstructured) Capability {
	if sc == nil || nilClient(dyn) {
		return unknown("capability_client_unavailable", "快照类选择客户端不可用")
	}
	base := decision("supported", "ready", "快照类选择已核验")
	base.StorageClass, base.Driver = sc.Name, sc.Provisioner
	base.backingSnapshot = strings.EqualFold(sc.Parameters["backingSnapshot"], "true")
	finish := func(c Capability) Capability { c.StorageClass = sc.Name; c.Driver = sc.Provisioner; return c }
	if sc.DeletionTimestamp != nil {
		return finish(unknown("storage_class_not_ready", "存储类正在删除"))
	}
	preferred := ""
	profile, profileErr := dyn.Resource(schema.GroupVersionResource{Group: "cdi.kubevirt.io", Version: "v1beta1", Resource: "storageprofiles"}).Get(ctx, sc.Name, metav1.GetOptions{})
	if profileErr != nil && !apierrors.IsNotFound(profileErr) {
		return finish(unknown("storage_profile_read_failed", "无法核验StorageProfile的快照类选择"))
	}
	if profileErr == nil {
		var fieldErr error
		preferred, _, fieldErr = unstructured.NestedString(profile.Object, "status", "snapshotClass")
		if fieldErr != nil || profile.GetDeletionTimestamp() != nil {
			return finish(unknown("storage_profile_class_unverified", "StorageProfile快照类配置无法核验"))
		}
	}
	var matches, defaults []string
	policies := map[string]string{}
	for _, c := range classes {
		driver, found, e := unstructured.NestedString(c.Object, "driver")
		if e != nil {
			return finish(unknown("snapshot_class_invalid", "快照类配置无法解析"))
		}
		if !found || driver != sc.Provisioner || c.GetDeletionTimestamp() != nil {
			continue
		}
		matches = append(matches, c.GetName())
		policy, _, policyErr := unstructured.NestedString(c.Object, "deletionPolicy")
		if policyErr != nil {
			return finish(unknown("snapshot_class_invalid", "快照类删除策略无法解析"))
		}
		policies[c.GetName()] = policy
		if c.GetAnnotations()["snapshot.storage.kubernetes.io/is-default-class"] == "true" {
			defaults = append(defaults, c.GetName())
		}
	}
	sort.Strings(matches)
	switch {
	case preferred != "":
		found := false
		for _, candidate := range matches {
			if candidate == preferred {
				found = true
				break
			}
		}
		if !found {
			return finish(unknown("storage_profile_class_unverified", "StorageProfile指定的快照类不存在或驱动不匹配"))
		}
		base.VolumeSnapshotClass = preferred
	case len(matches) == 0:
		return finish(decision("unsupported", "snapshot_class_missing", "没有与该CSI驱动匹配的v1 VolumeSnapshotClass"))
	case len(matches) == 1:
		base.VolumeSnapshotClass = matches[0]
	case len(defaults) == 1:
		base.VolumeSnapshotClass = defaults[0]
	default:
		return finish(unknown("snapshot_class_ambiguous", "存在多个匹配快照类且无法唯一确定默认类"))
	}
	base.deletionPolicy = policies[base.VolumeSnapshotClass]
	return base
}

// Volumes aggregates all participants. Callers must not pass only the root disk.
func (i *Inspector) Volumes(ctx context.Context, volumes []Volume) SnapshotSet {
	out := SnapshotSet{Capability: decision("supported", "ready", "所有参与磁盘均具备快照配置与就绪证据"), Volumes: []VolumeCapability{}}
	if len(volumes) == 0 {
		out.Capability = unknown("volume_set_empty", "没有可核验的快照参与磁盘")
		return out
	}
	seen := map[string]bool{}
	for _, v := range volumes {
		c := i.StorageClass(ctx, v.StorageClass)
		if v.Key == "" || seen[v.Key] {
			c = unknown("volume_set_invalid", "磁盘集合存在缺失或重复身份")
		}
		seen[v.Key] = true
		out.Volumes = append(out.Volumes, VolumeCapability{Key: v.Key, Capability: c})
		if c.Status == "unsupported" {
			out.Capability = decision("unsupported", "participating_volume_unsupported", "至少一块参与磁盘不支持快照，请查看逐盘原因")
		} else if !c.Supported && out.Status != "unsupported" {
			out.Capability = unknown("participating_volume_unverified", "至少一块参与磁盘无法确认快照能力，请查看逐盘原因")
		}
	}
	return out
}
func podReady(p *corev1.Pod) bool {
	if p.DeletionTimestamp != nil || p.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
func containerReady(p *corev1.Pod, name string) bool {
	for _, c := range p.Status.ContainerStatuses {
		if c.Name == name {
			return c.Ready && c.State.Running != nil
		}
	}
	return false
}
func component(c corev1.Container, name string) bool {
	for _, a := range c.Command {
		if path.Base(a) == name {
			return true
		}
	}
	image := strings.Split(strings.Split(path.Base(c.Image), "@")[0], ":")[0]
	return image == name
}
func driverName(c corev1.Container) string {
	args := append(append([]string{}, c.Command...), c.Args...)
	for n, a := range args {
		var value string
		for _, flag := range []string{"--drivername", "--driver-name"} {
			if strings.HasPrefix(a, flag+"=") {
				value = strings.TrimPrefix(a, flag+"=")
			}
			if a == flag && n+1 < len(args) {
				value = args[n+1]
			}
		}
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "$(") && strings.HasSuffix(value, ")") {
			name := strings.TrimSuffix(strings.TrimPrefix(value, "$("), ")")
			for _, e := range c.Env {
				if e.Name == name && e.ValueFrom == nil {
					return e.Value
				}
			}
			return ""
		}
		return value
	}
	return ""
}

func csiSocketHostPaths(p *corev1.Pod, c corev1.Container) []string {
	socket := containerFlagValue(c, "--csi-address", "--endpoint")
	socket = strings.TrimPrefix(socket, "unix://")
	if !strings.HasPrefix(socket, "/") {
		return nil
	}
	volumes := map[string]*corev1.HostPathVolumeSource{}
	for n := range p.Spec.Volumes {
		volume := &p.Spec.Volumes[n]
		if volume.HostPath != nil {
			volumes[volume.Name] = volume.HostPath
		}
	}
	result := []string{}
	for _, mount := range c.VolumeMounts {
		root := strings.TrimSuffix(path.Clean(mount.MountPath), "/")
		if root == "." || socket != root && !strings.HasPrefix(socket, root+"/") {
			continue
		}
		host := volumes[mount.Name]
		if host == nil || !strings.HasPrefix(host.Path, "/") {
			continue
		}
		relative := strings.TrimPrefix(strings.TrimPrefix(socket, root), "/")
		result = append(result, path.Join(host.Path, relative))
	}
	return result
}

func containerFlagValue(c corev1.Container, flags ...string) string {
	args := append(append([]string{}, c.Command...), c.Args...)
	for n, argument := range args {
		value := ""
		for _, flag := range flags {
			if strings.HasPrefix(argument, flag+"=") {
				value = strings.TrimPrefix(argument, flag+"=")
			} else if argument == flag && n+1 < len(args) {
				value = args[n+1]
			}
		}
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "$(") && strings.HasSuffix(value, ")") {
			name := strings.TrimSuffix(strings.TrimPrefix(value, "$("), ")")
			for _, env := range c.Env {
				if env.Name == name && env.ValueFrom == nil {
					return env.Value
				}
			}
			return ""
		}
		return value
	}
	return ""
}

func nilClient(v interface{}) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	return r.Kind() == reflect.Ptr && r.IsNil()
}
