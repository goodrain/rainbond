package handler

import (
	"context"
	"encoding/json"
)

type kubeletFilesystemStats struct {
	CapacityBytes  *uint64 `json:"capacityBytes"`
	AvailableBytes *uint64 `json:"availableBytes"`
	UsedBytes      *uint64 `json:"usedBytes"`
}

type kubeletStatsSummary struct {
	Node struct {
		Fs      *kubeletFilesystemStats `json:"fs"`
		Runtime struct {
			ImageFs     *kubeletFilesystemStats `json:"imageFs"`
			ContainerFs *kubeletFilesystemStats `json:"containerFs"`
		} `json:"runtime"`
	} `json:"node"`
}

type filesystemUsage struct {
	capacityBytes uint64
	usedBytes     uint64
}

type nodeFilesystemStats struct {
	root      filesystemUsage
	container filesystemUsage
}

func (n *nodesHandle) getNodeFilesystemStats(ctx context.Context, nodeName string) (nodeFilesystemStats, error) {
	raw, err := n.clientset.CoreV1().RESTClient().Get().
		Resource("nodes").
		Name(nodeName).
		SubResource("proxy").
		Suffix("stats", "summary").
		DoRaw(ctx)
	if err != nil {
		return nodeFilesystemStats{}, err
	}
	return parseNodeFilesystemStats(raw)
}

func parseNodeFilesystemStats(raw []byte) (nodeFilesystemStats, error) {
	var summary kubeletStatsSummary
	if err := json.Unmarshal(raw, &summary); err != nil {
		return nodeFilesystemStats{}, err
	}

	return nodeFilesystemStats{
		root: filesystemUsageFromKubelet(summary.Node.Fs),
		container: mostUtilizedFilesystem(
			filesystemUsageFromKubelet(summary.Node.Runtime.ImageFs),
			filesystemUsageFromKubelet(summary.Node.Runtime.ContainerFs),
		),
	}, nil
}

func filesystemUsageFromKubelet(stats *kubeletFilesystemStats) filesystemUsage {
	if stats == nil || stats.CapacityBytes == nil || *stats.CapacityBytes == 0 {
		return filesystemUsage{}
	}

	capacity := *stats.CapacityBytes
	if stats.AvailableBytes != nil && *stats.AvailableBytes <= capacity {
		return filesystemUsage{
			capacityBytes: capacity,
			usedBytes:     capacity - *stats.AvailableBytes,
		}
	}
	if stats.UsedBytes == nil {
		return filesystemUsage{}
	}

	used := *stats.UsedBytes
	if used > capacity {
		used = capacity
	}
	return filesystemUsage{capacityBytes: capacity, usedBytes: used}
}

func mostUtilizedFilesystem(filesystems ...filesystemUsage) filesystemUsage {
	var selected filesystemUsage
	for _, filesystem := range filesystems {
		if filesystem.capacityBytes == 0 {
			continue
		}
		if selected.capacityBytes == 0 ||
			float64(filesystem.usedBytes)/float64(filesystem.capacityBytes) >
				float64(selected.usedBytes)/float64(selected.capacityBytes) {
			selected = filesystem
		}
	}
	return selected
}
