package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/prometheus/common/log"
	"github.com/urfave/cli"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"strconv"
	"text/tabwriter"
	"time"
)

//DeviceInfo -
type DeviceInfo struct {
	idx         int
	pods        []v1.Pod
	usedGPUMem  int
	totalGPUMem int
	node        v1.Node
}

//NodeInfo -
type NodeInfo struct {
	pods           []v1.Pod
	node           v1.Node
	devs           map[int]*DeviceInfo
	gpuCount       int
	gpuTotalMemory int
	pluginPod      v1.Pod
}

var (
	retries    = 5
	memoryUnit = ""
)

//NewCmdGPUShare -
func NewCmdGPUShare() cli.Command {
	c := cli.Command{
		Name:  "gpushare",
		Usage: "display gpu share information",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "detail",
				Usage: "details",
			},
		},
		Action: func(c *cli.Context) error {
			Common(c)
			return showGPUShare(c)
		},
	}
	return c
}

func showGPUShare(c *cli.Context) error {

	var pods []v1.Pod
	var nodes []v1.Node
	var err error
	nodes, err = getAllSharedGPUNode()
	if err == nil {
		pods, err = getActivePodsInAllNodes()
	}

	if err != nil {
		fmt.Printf("Failed due to %v", err)
		return err
	}

	nodeInfos, err := buildAllNodeInfos(pods, nodes)
	if err != nil {
		fmt.Printf("Failed due to %v", err)
		return err
	}
	if c.Bool("detail") {
		displayDetails(nodeInfos)
	} else {
		displaySummary(nodeInfos)
	}
	return nil
}

const (
	resourceName           = "rainbond.com/gpu-mem"
	countName              = "rainbond.com/gpu-count"
	envNVGPUID             = "RAINBOND_COM_GPU_MEM_IDX"
	gpushareAllocationFlag = "scheduler.framework.gpushare.allocation"
)

func getAllSharedGPUNode() ([]v1.Node, error) {
	nodes := []v1.Node{}
	allNodes, err := clients.K8SClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nodes, err
	}

	for _, item := range allNodes.Items {
		if isGPUSharingNode(item) {
			nodes = append(nodes, item)
		}
	}

	return nodes, nil
}

func isGPUSharingNode(node v1.Node) bool {
	value, ok := node.Status.Allocatable[resourceName]

	if ok {
		ok = (int(value.Value()) > 0)
	}

	return ok
}

func getActivePodsInAllNodes() ([]v1.Pod, error) {
	pods, err := clients.K8SClient.CoreV1().Pods(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
	})

	for i := 0; i < retries && err != nil; i++ {
		pods, err = clients.K8SClient.CoreV1().Pods(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{
			LabelSelector: labels.Everything().String(),
		})
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return []v1.Pod{}, fmt.Errorf("failed to get Pods")
	}
	return filterActivePods(pods.Items), nil
}

func filterActivePods(pods []v1.Pod) (activePods []v1.Pod) {
	activePods = []v1.Pod{}
	for _, pod := range pods {
		if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
			continue
		}

		activePods = append(activePods, pod)
	}

	return activePods
}

func buildAllNodeInfos(allPods []v1.Pod, nodes []v1.Node) ([]*NodeInfo, error) {
	nodeInfos := buildNodeInfoWithPods(allPods, nodes)
	for _, info := range nodeInfos {
		if info.gpuTotalMemory > 0 {
			setUnit(info.gpuTotalMemory, info.gpuCount)
			err := info.buildDeviceInfo()
			if err != nil {
				continue
			}
		}
	}
	return nodeInfos, nil
}

func setUnit(gpuMemory, gpuCount int) {
	if memoryUnit != "" {
		return
	}

	if gpuCount == 0 {
		return
	}

	gpuMemoryByDev := gpuMemory / gpuCount

	if gpuMemoryByDev > 100 {
		memoryUnit = "MiB"
	} else {
		memoryUnit = "GiB"
	}
}

func buildNodeInfoWithPods(pods []v1.Pod, nodes []v1.Node) []*NodeInfo {
	nodeMap := map[string]*NodeInfo{}
	nodeList := []*NodeInfo{}

	for _, node := range nodes {
		var info *NodeInfo = &NodeInfo{}
		if value, ok := nodeMap[node.Name]; ok {
			info = value
		} else {
			nodeMap[node.Name] = info
			info.node = node
			info.pods = []v1.Pod{}
			info.gpuCount = getGPUCountInNode(node)
			info.gpuTotalMemory = getTotalGPUMemory(node)
			info.devs = map[int]*DeviceInfo{}

			for i := 0; i < info.gpuCount; i++ {
				dev := &DeviceInfo{
					pods:        []v1.Pod{},
					idx:         i,
					totalGPUMem: info.gpuTotalMemory / info.gpuCount,
					node:        info.node,
				}
				info.devs[i] = dev
			}

		}

		for _, pod := range pods {
			if pod.Spec.NodeName == node.Name {
				info.pods = append(info.pods, pod)
			}
		}
	}

	for _, v := range nodeMap {
		nodeList = append(nodeList, v)
	}
	return nodeList
}

func displayDetails(nodeInfos []*NodeInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var (
		totalGPUMemInCluster int64
		usedGPUMemInCluster  int64
		prtLineLen           int
	)

	for _, nodeInfo := range nodeInfos {
		address := "unknown"
		if len(nodeInfo.node.Status.Addresses) > 0 {
			for _, addr := range nodeInfo.node.Status.Addresses {
				if addr.Type == v1.NodeInternalIP {
					address = addr.Address
					break
				}
			}
		}

		totalGPUMemInNode := nodeInfo.gpuTotalMemory
		if totalGPUMemInNode <= 0 {
			continue
		}

		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "NAME:\t%s\n", nodeInfo.node.Name)
		fmt.Fprintf(w, "IPADDRESS:\t%s\n", address)
		fmt.Fprintf(w, "\n")

		usedGPUMemInNode := 0
		var buf bytes.Buffer
		buf.WriteString("NAME\tNAMESPACE\t")
		for i := 0; i < nodeInfo.gpuCount; i++ {
			buf.WriteString(fmt.Sprintf("GPU%d(Allocated)\t", i))
		}

		if nodeInfo.hasPendingGPUMemory() {
			buf.WriteString("Pending(Allocated)\t")
		}
		buf.WriteString("\n")
		fmt.Fprintf(w, buf.String())

		var buffer bytes.Buffer
		exists := map[types.UID]bool{}
		for i, dev := range nodeInfo.devs {
			usedGPUMemInNode += dev.usedGPUMem
			for _, pod := range dev.pods {
				if _, ok := exists[pod.UID]; ok {
					continue
				}
				buffer.WriteString(fmt.Sprintf("%s\t%s\t", pod.Name, pod.Namespace))
				count := nodeInfo.gpuCount
				if nodeInfo.hasPendingGPUMemory() {
					count++
				}

				for k := 0; k < count; k++ {
					allocation := GetAllocation(&pod)
					if len(allocation) != 0 {
						buffer.WriteString(fmt.Sprintf("%d\t", allocation[k]))
						continue
					}
					if k == i || (i == -1 && k == nodeInfo.gpuCount) {
						buffer.WriteString(fmt.Sprintf("%d\t", getGPUMemoryInPod(pod)))
					} else {
						buffer.WriteString("0\t")
					}
				}
				buffer.WriteString("\n")
				exists[pod.UID] = true
			}
		}
		if prtLineLen == 0 {
			prtLineLen = buffer.Len() + 10
		}
		fmt.Fprintf(w, buffer.String())

		var gpuUsageInNode float64 = 0
		if totalGPUMemInNode > 0 {
			gpuUsageInNode = float64(usedGPUMemInNode) / float64(totalGPUMemInNode) * 100
		} else {
			fmt.Fprintf(w, "\n")
		}

		fmt.Fprintf(w, "Allocated :\t%d (%d%%)\t\n", usedGPUMemInNode, int64(gpuUsageInNode))
		fmt.Fprintf(w, "Total :\t%d \t\n", nodeInfo.gpuTotalMemory)
		var prtLine bytes.Buffer
		for i := 0; i < prtLineLen; i++ {
			prtLine.WriteString("-")
		}
		prtLine.WriteString("\n")
		fmt.Fprintf(w, prtLine.String())
		totalGPUMemInCluster += int64(totalGPUMemInNode)
		usedGPUMemInCluster += int64(usedGPUMemInNode)
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "Allocated/Total GPU Memory In Cluster:\t")
	var gpuUsage float64 = 0
	if totalGPUMemInCluster > 0 {
		gpuUsage = float64(usedGPUMemInCluster) / float64(totalGPUMemInCluster) * 100
	}
	fmt.Fprintf(w, "%s/%s (%d%%)\t\n",
		strconv.FormatInt(usedGPUMemInCluster, 10),
		strconv.FormatInt(totalGPUMemInCluster, 10),
		int64(gpuUsage))
	_ = w.Flush()
}

func displaySummary(nodeInfos []*NodeInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var (
		maxGPUCount          int
		totalGPUMemInCluster int64
		usedGPUMemInCluster  int64
		prtLineLen           int
	)

	hasPendingGPU := hasPendingGPUMemory(nodeInfos)

	maxGPUCount = getMaxGPUCount(nodeInfos)

	var buffer bytes.Buffer
	buffer.WriteString("NAME\tIPADDRESS\t")
	for i := 0; i < maxGPUCount; i++ {
		buffer.WriteString(fmt.Sprintf("GPU%d(Allocated/Total)\t", i))
	}

	if hasPendingGPU {
		buffer.WriteString("PENDING(Allocated)\t")
	}
	buffer.WriteString(fmt.Sprintf("GPU Memory(%s)\n", memoryUnit))

	fmt.Fprintf(w, buffer.String())
	for _, nodeInfo := range nodeInfos {
		address := "unknown"
		if len(nodeInfo.node.Status.Addresses) > 0 {
			for _, addr := range nodeInfo.node.Status.Addresses {
				if addr.Type == v1.NodeInternalIP {
					address = addr.Address
					break
				}
			}
		}

		gpuMemInfos := []string{}
		pendingGPUMemInfo := ""
		usedGPUMemInNode := 0
		totalGPUMemInNode := nodeInfo.gpuTotalMemory
		if totalGPUMemInNode <= 0 {
			continue
		}

		for i := 0; i < maxGPUCount; i++ {
			gpuMemInfo := "0/0"
			if dev, ok := nodeInfo.devs[i]; ok {
				gpuMemInfo = dev.String()
				usedGPUMemInNode += dev.usedGPUMem
			}
			gpuMemInfos = append(gpuMemInfos, gpuMemInfo)
		}
		if dev, ok := nodeInfo.devs[-1]; ok {
			pendingGPUMemInfo = fmt.Sprintf("%d", dev.usedGPUMem)
			usedGPUMemInNode += dev.usedGPUMem
		}

		nodeGPUMemInfo := fmt.Sprintf("%d/%d", usedGPUMemInNode, totalGPUMemInNode)

		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("%s\t%s\t", nodeInfo.node.Name, address))
		for i := 0; i < maxGPUCount; i++ {
			buf.WriteString(fmt.Sprintf("%s\t", gpuMemInfos[i]))
		}
		if hasPendingGPU {
			buf.WriteString(fmt.Sprintf("%s\t", pendingGPUMemInfo))
		}

		buf.WriteString(fmt.Sprintf("%s\n", nodeGPUMemInfo))
		fmt.Fprintf(w, buf.String())

		if prtLineLen == 0 {
			prtLineLen = buf.Len() + 20
		}

		usedGPUMemInCluster += int64(usedGPUMemInNode)
		totalGPUMemInCluster += int64(totalGPUMemInNode)
	}
	var prtLine bytes.Buffer
	for i := 0; i < prtLineLen; i++ {
		prtLine.WriteString("-")
	}
	prtLine.WriteString("\n")
	fmt.Fprint(w, prtLine.String())

	fmt.Fprintf(w, "Allocated/Total GPU Memory In Cluster:\n")
	var gpuUsage float64 = 0
	if totalGPUMemInCluster > 0 {
		gpuUsage = float64(usedGPUMemInCluster) / float64(totalGPUMemInCluster) * 100
	}
	fmt.Fprintf(w, "%s/%s (%d%%)\t\n",
		strconv.FormatInt(usedGPUMemInCluster, 10),
		strconv.FormatInt(totalGPUMemInCluster, 10),
		int64(gpuUsage))

	_ = w.Flush()
}

func (n *NodeInfo) buildDeviceInfo() error {
	totalGPUMem := 0
	if n.gpuCount > 0 {
		totalGPUMem = n.gpuTotalMemory / n.gpuCount
	}
GPUSearchLoop:
	for _, pod := range n.pods {
		if gpuMemoryInPod(pod) <= 0 {
			continue GPUSearchLoop
		}
		for devID, usedGPUMem := range n.getDeivceInfo(pod) {
			if n.devs[devID] == nil {
				n.devs[devID] = &DeviceInfo{
					pods:        []v1.Pod{},
					idx:         devID,
					totalGPUMem: totalGPUMem,
					node:        n.node,
				}
			}
			n.devs[devID].usedGPUMem += usedGPUMem
			n.devs[devID].pods = append(n.devs[devID].pods, pod)
		}
	}
	return nil
}

func getGPUCountInNode(node v1.Node) int {
	val, ok := node.Status.Allocatable[countName]

	if !ok {
		return int(0)
	}

	return int(val.Value())
}

func getTotalGPUMemory(node v1.Node) int {
	val, ok := node.Status.Allocatable[resourceName]

	if !ok {
		return 0
	}

	return int(val.Value())
}

func hasPendingGPUMemory(nodeInfos []*NodeInfo) (found bool) {
	for _, info := range nodeInfos {
		if info.hasPendingGPUMemory() {
			return true
		}
	}

	return false
}

func (n *NodeInfo) hasPendingGPUMemory() bool {
	_, found := n.devs[-1]
	return found
}

//GetAllocation -
func GetAllocation(pod *v1.Pod) map[int]int {
	podGPUMems := map[int]int{}
	allocationString := ""
	if pod.ObjectMeta.Annotations == nil {
		return podGPUMems
	}
	value, ok := pod.ObjectMeta.Annotations[gpushareAllocationFlag]
	if !ok {
		return podGPUMems
	}
	allocationString = value
	var allocation map[int]map[string]int
	err := json.Unmarshal([]byte(allocationString), &allocation)
	if err != nil {
		return podGPUMems
	}
	for _, containerAllocation := range allocation {
		for id, gpuMem := range containerAllocation {
			gpuIndex, err := strconv.Atoi(id)
			if err != nil {
				log.Errorf("failed to get gpu memory from pod annotation,reason: %v", err)
				return map[int]int{}
			}
			podGPUMems[gpuIndex] += gpuMem
		}
	}
	return podGPUMems
}

func getGPUMemoryInPod(pod v1.Pod) int {
	gpuMem := 0
	for _, container := range pod.Spec.Containers {
		if val, ok := container.Resources.Limits[resourceName]; ok {
			gpuMem += int(val.Value())
		}
	}
	return gpuMem
}

func getMaxGPUCount(nodeInfos []*NodeInfo) (max int) {
	for _, node := range nodeInfos {
		if node.gpuCount > max {
			max = node.gpuCount
		}
	}

	return max
}

func (d *DeviceInfo) String() string {
	if d.idx == -1 {
		return fmt.Sprintf("%d", d.usedGPUMem)
	}
	return fmt.Sprintf("%d/%d", d.usedGPUMem, d.totalGPUMem)
}

func gpuMemoryInPod(pod v1.Pod) int {
	var total int
	containers := pod.Spec.Containers
	for _, container := range containers {
		if val, ok := container.Resources.Limits[resourceName]; ok {
			total += int(val.Value())
		}
	}

	return total
}

func (n *NodeInfo) getDeivceInfo(pod v1.Pod) map[int]int {
	var err error
	id := -1
	allocation := map[int]int{}
	allocation = GetAllocation(&pod)
	if len(allocation) != 0 {
		return allocation
	}
	if len(pod.ObjectMeta.Annotations) > 0 {
		value, found := pod.ObjectMeta.Annotations[envNVGPUID]
		if found {
			id, err = strconv.Atoi(value)
			if err != nil {
				fmt.Printf("Failed to parse dev id %s due to %v for pod %s in ns %s",
					value,
					err,
					pod.Name,
					pod.Namespace)
				id = -1
			}
		} else {
			fmt.Printf("Failed to get dev id %s for pod %s in ns %s",
				pod.Name,
				pod.Namespace)
		}
	}
	allocation[id] = gpuMemoryInPod(pod)
	return allocation
}
