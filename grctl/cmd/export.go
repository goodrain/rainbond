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
	"path/filepath"
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
	namespaceList, err := clients.K8SClient.CoreV1().Namespaces().List(context.Background(), v1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range namespaceList.Items {
		namespace := namespaceList.Items[i]
		if namespace.Name == "kube-system" || namespace.Name == "kube-public" || namespace.Name == "rainbond" {
			continue
		}
		err := exportNamespace(namespace.Name)
		if err != nil {
			fmt.Println(fmt.Errorf("export namespace [%s] error: %s", namespace.Name, err.Error()))
		}
	}
	return nil
}

func exportNamespace(namespace string) error {
	list, err := clients.K8SClient.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: "creator=Rainbond",
	})
	if err != nil {
		return err
	}
	if len(list.Items) == 0 {
		return nil
	}

	volumeList, err := clients.K8SClient.CoreV1().PersistentVolumes().List(context.Background(), v1.ListOptions{})
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	f, err := os.Create("pvc.tgz")

	// Wrap in gzip writer
	zipper := gzip.NewWriter(f)
	zipper.Header.Extra = []byte("+aHR0cHM6Ly95b3V0dS5iZS96OVV6MWljandyTQo=")
	zipper.Header.Comment = "Rainbond"

	// Wrap in tar writer
	twriter := tar.NewWriter(zipper)
	defer func() {
		twriter.Close()
		zipper.Close()
		f.Close()
	}()

	for _, pvc := range list.Items {
		pvc.ManagedFields = nil
		pvc.OwnerReferences = nil
		pvc.CreationTimestamp = v1.Time{}
		pvc.UID = ""
		pvc.ResourceVersion = ""
		pvc.Status = corev1.PersistentVolumeClaimStatus{}
		pvc.Kind = "PersistentVolumeClaim"
		pvc.APIVersion = "v1"

		for _, pv := range volumeList.Items {
			if pvc.Spec.VolumeName == pv.Name {
				pv.ManagedFields = nil
				pv.ManagedFields = nil
				pv.OwnerReferences = nil
				pv.CreationTimestamp = v1.Time{}
				pv.UID = ""
				pv.ResourceVersion = ""
				pv.Status = corev1.PersistentVolumeStatus{}
				pv.Kind = "PersistentVolume"
				pv.APIVersion = "v1"
				body, _ := yaml.Marshal(pv)
				_ = writeToTar(twriter, fmt.Sprintf("pv/%s/%s.yaml", namespace, pv.Name), body)
			}
		}

		body, _ := yaml.Marshal(pvc)
		_ = writeToTar(twriter, fmt.Sprintf("pvc/%s/%s.yaml", namespace, pvc.Name), body)
		fmt.Println(fmt.Sprintf("export persistentVolumeClaim namespace: [%s] name: [%s]", pvc.Namespace, pvc.Name))
	}
	return nil
}

// writeToTar writes a single file to a tar archive.
func writeToTar(out *tar.Writer, name string, body []byte) error {
	// TODO: Do we need to create dummy parent directory names if none exist?
	h := &tar.Header{
		Name:    filepath.ToSlash(name),
		Mode:    0644,
		Size:    int64(len(body)),
		ModTime: time.Now(),
	}
	if err := out.WriteHeader(h); err != nil {
		return err
	}
	_, err := out.Write(body)
	return err
}
