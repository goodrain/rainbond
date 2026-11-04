package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/goodrain/rainbond/api/model"
	"github.com/shirou/gopsutil/v4/disk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const clusterInfoQueryTimeout = 3 * time.Second

const clusterLocalDiskSampleTTL = 30 * time.Second

type clusterInfoRefresh struct {
	done   chan struct{}
	result *model.ClusterResource
	err    error
}

type clusterLocalDiskSample struct {
	capacity, used         uint64
	available, failed      bool
	refreshing             bool
	sampledAt, lastAttempt time.Time
}

// loadClusterInfo uses one Node list and one workload Pod list, independent of
// exporter availability. Requests are scheduled Rainbond workloads, not actual
// usage, and node capacity remains the whole cluster's allocatable capacity.
func (c *clusterAction) loadClusterInfo(ctx context.Context) (*model.ClusterResource, error) {
	nodes, err := c.listNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("get cluster nodes: %w", err)
	}
	pods, err := c.clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{LabelSelector: "creator=Rainbond"})
	if err != nil {
		return nil, fmt.Errorf("get cluster workload pods: %w", err)
	}
	result := buildClusterResourceSnapshot(nodes, pods.Items)

	// Version is optional and has a shorter deadline than the resource reads.
	versionCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	if raw, err := c.clientset.Discovery().RESTClient().Get().AbsPath("/version").DoRaw(versionCtx); err == nil {
		var version struct {
			GitVersion string `json:"gitVersion"`
		}
		if json.Unmarshal(raw, &version) == nil {
			result.K8sVersion = version.GitVersion
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// withLocalDiskStats returns a copy, so background sampling never mutates a
// Kubernetes snapshot or a response already being encoded. Unix statfs cannot
// be canceled: retain at most one background call, including if it hangs.
func (c *clusterAction) withLocalDiskStats(result *model.ClusterResource) *model.ClusterResource {
	if result == nil {
		return nil
	}
	response := *result
	response.LocalDiskStatsStatus = "unavailable"
	now := time.Now()
	c.cacheMu.Lock()
	sample := c.localDisk
	if sample.available {
		response.CapDisk, response.ReqDisk = sample.capacity, sample.used
		response.LocalDiskStatsStatus = "ok"
		if sample.failed || !sample.sampledAt.Add(clusterLocalDiskSampleTTL).After(now) {
			response.LocalDiskStatsStatus = "stale"
		}
	}
	refreshAfter := sample.lastAttempt.Add(clusterLocalDiskSampleTTL)
	if sample.available && !sample.failed {
		refreshAfter = sample.sampledAt.Add(clusterLocalDiskSampleTTL)
	}
	if !sample.refreshing && !refreshAfter.After(now) {
		c.localDisk.refreshing = true
		c.localDisk.lastAttempt = now
		readDisk := c.diskUsage
		if readDisk == nil {
			readDisk = disk.Usage
		}
		go c.refreshLocalDisk(readDisk)
	}
	c.cacheMu.Unlock()
	return &response
}

func (c *clusterAction) refreshLocalDisk(readDisk func(string) (*disk.UsageStat, error)) {
	path := "/grdata"
	if runtime.GOOS == "windows" {
		path = `z:\\`
	}
	usage, err := readDisk(path)
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.localDisk.refreshing = false
	c.localDisk.failed = err != nil || usage == nil || usage.Total == 0 || usage.Used > usage.Total
	if !c.localDisk.failed {
		c.localDisk.capacity, c.localDisk.used = usage.Total, usage.Used
		c.localDisk.sampledAt = time.Now()
		c.localDisk.available = true
	}
}

func buildClusterResourceSnapshot(nodes []*corev1.Node, pods []corev1.Pod) *model.ClusterResource {
	result := &model.ClusterResource{
		AllNode: len(nodes), ComputeNode: len(nodes), ResourceProxyStatus: true,
		ResourceSource: "kubernetes", ResourceScope: "rainbond_workloads",
	}
	nodeMap := make(map[string]*corev1.Node, len(nodes))
	resources := make(map[string]*model.NodeResource, len(nodes))
	for _, node := range nodes {
		nodeMap[node.Name] = node
		resources[node.Name] = model.NewResource(node.Status.Allocatable)
		result.CapCPUMillicores += node.Status.Allocatable.Cpu().MilliValue()
	}
	var healthyCPU, unhealthyCPU, healthyMemory, unhealthyMemory int64
	for i := range pods {
		pod := &pods[i]
		node := nodeMap[pod.Spec.NodeName]
		if node == nil || isTerminalPod(pod) || pod.Labels["creator"] != "Rainbond" {
			continue
		}
		info := podResourceInformationFromPod(pod)
		result.RunPodNumber++ // Legacy: scheduled non-terminal workloads.
		if pod.Status.Phase == corev1.PodRunning {
			result.RunningPodNumber++
		}
		if isNodeReady(node) {
			healthyCPU += info.CPU
			healthyMemory += info.Memory
		} else {
			unhealthyCPU += info.CPU
			unhealthyMemory += info.Memory
		}
		available := resources[node.Name]
		available.AllowedPodNumber--
		available.MilliCPU -= info.CPU
		available.Memory -= info.Memory
		available.EphemeralStorage -= info.StorageEphemeral
	}
	var healthCapMemory, unhealthCapMemory int64
	for _, node := range nodes {
		if isNodeReady(node) {
			result.NodeReady++
			result.HealthCapCPU += int(node.Status.Allocatable.Cpu().Value())
			healthCapMemory += node.Status.Allocatable.Memory().Value()
		} else {
			result.NotReadyNode++
			result.UnhealthCapCPU += int(node.Status.Allocatable.Cpu().Value())
			unhealthCapMemory += node.Status.Allocatable.Memory().Value()
		}
		if !isNodeReady(node) || node.Spec.Unschedulable || containsTaints(node) {
			continue
		}
		available := resources[node.Name]
		if result.MaxAllocatableMemoryNodeResource == nil || available.Memory > result.MaxAllocatableMemoryNodeResource.Memory {
			result.MaxAllocatableMemoryNodeResource = available
		}
	}
	result.CapCPU = result.HealthCapCPU + result.UnhealthCapCPU
	result.CapMem = int((healthCapMemory + unhealthCapMemory) / (1024 * 1024))
	result.HealthCapMem = int(healthCapMemory / (1024 * 1024))
	result.UnhealthCapMem = int(unhealthCapMemory / (1024 * 1024))
	result.HealthReqCPU = float32(healthyCPU) / 1000
	result.UnhealthReqCPU = float32(unhealthyCPU) / 1000
	result.ReqCPU = float32(healthyCPU+unhealthyCPU) / 1000
	result.HealthReqMem = int(healthyMemory / (1024 * 1024))
	result.UnhealthReqMem = int(unhealthyMemory / (1024 * 1024))
	result.ReqMem = int((healthyMemory + unhealthyMemory) / (1024 * 1024))
	result.RainbondReqCPU = result.ReqCPU
	result.RainbondReqMem = result.ReqMem
	return result
}
