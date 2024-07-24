package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/urfave/cli"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"sigs.k8s.io/yaml"
	"time"
)

// NewCmdExport -
func NewCmdExport() cli.Command {
	c := cli.Command{
		Name:  "export",
		Usage: "this command is used to export the rainbond resource\n",
		Action: func(c *cli.Context) error {
			Common(c)
			return export()
		},
	}
	return c
}

func export() error {
	// 打开输出文件
	f, err := os.Create("pvc_pv_export.tgz")
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println("Error closing file:", err)
		}
	}()

	// 创建gzip和tar writers
	zipper := gzip.NewWriter(f)
	defer func() {
		if err := zipper.Close(); err != nil {
			fmt.Println("Error closing gzip writer:", err)
		}
	}()
	zipper.Header.Extra = []byte("+aHR0cHM6Ly95b3V0dS5iZS96OVV6MWljandyTQo=")
	zipper.Header.Comment = "Rainbond"

	twriter := tar.NewWriter(zipper)
	defer func() {
		if err := twriter.Close(); err != nil {
			fmt.Println("Error closing tar writer:", err)
		}
	}()

	// 获取所有命名空间列表
	namespaceList, err := clients.K8SClient.CoreV1().Namespaces().List(context.Background(), v1.ListOptions{})
	if err != nil {
		return err
	}

	// 遍历每个命名空间
	for _, namespace := range namespaceList.Items {
		if namespace.Name == "kube-system" || namespace.Name == "kube-public" || namespace.Name == "rainbond" {
			continue
		}
		err := exportNamespace(namespace.Name, twriter)
		if err != nil {
			fmt.Println(fmt.Errorf("export namespace [%s] error: %s", namespace.Name, err.Error()))
		}
	}
	return nil
}

func exportNamespace(namespace string, twriter *tar.Writer) error {
	// 获取命名空间中的PVC列表
	list, err := clients.K8SClient.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: "creator=Rainbond",
	})
	if err != nil {
		return err
	}
	if len(list.Items) == 0 {
		return nil
	}

	// 获取所有PV列表
	volumeList, err := clients.K8SClient.CoreV1().PersistentVolumes().List(context.Background(), v1.ListOptions{})
	if err != nil {
		return err
	}

	// 遍历每个PVC
	for _, pvc := range list.Items {
		cleanPVC(&pvc)
		// 查找与PVC关联的PV
		for _, pv := range volumeList.Items {
			if pvc.Spec.VolumeName == pv.Name {
				cleanPV(&pv)
				body, err := yaml.Marshal(pv)
				if err != nil {
					fmt.Println("Error marshalling PV:", err)
					continue
				}
				if err := writeToTar(twriter, fmt.Sprintf("pv/%s/%s.yaml", namespace, pv.Name), body); err != nil {
					fmt.Println("Error writing PV to tar:", err)
				}
			}
		}

		// 将PVC写入tar包
		body, err := yaml.Marshal(pvc)
		if err != nil {
			fmt.Println("Error marshalling PVC:", err)
			continue
		}
		if err := writeToTar(twriter, fmt.Sprintf("pvc/%s/%s.yaml", namespace, pvc.Name), body); err != nil {
			fmt.Println("Error writing PVC to tar:", err)
		}
		fmt.Printf("Exported persistentVolumeClaim namespace: [%s] name: [%s]\n", pvc.Namespace, pvc.Name)
	}
	return nil
}

func cleanPVC(pvc *corev1.PersistentVolumeClaim) {
	pvc.ManagedFields = nil
	pvc.OwnerReferences = nil
	pvc.CreationTimestamp = v1.Time{}
	pvc.UID = ""
	pvc.ResourceVersion = ""
	pvc.Status = corev1.PersistentVolumeClaimStatus{}
	pvc.Kind = "PersistentVolumeClaim"
	pvc.APIVersion = "v1"
}

func cleanPV(pv *corev1.PersistentVolume) {
	pv.ManagedFields = nil
	pv.OwnerReferences = nil
	pv.CreationTimestamp = v1.Time{}
	pv.UID = ""
	pv.ResourceVersion = ""
	pv.Status = corev1.PersistentVolumeStatus{}
	pv.Kind = "PersistentVolume"
	pv.APIVersion = "v1"
}

func writeToTar(tw *tar.Writer, name string, body []byte) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0600,
		Size:    int64(len(body)),
		ModTime: time.Now(), // 使用当前时间作为文件的修改时间
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(body); err != nil {
		return err
	}
	return nil
}
