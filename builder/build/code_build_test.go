package build

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/cmd/builder/option"
	"github.com/goodrain/rainbond/event"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	etcdutil "github.com/goodrain/rainbond/util/etcd"
	k8sutil "github.com/goodrain/rainbond/util/k8s"

	"k8s.io/client-go/kubernetes"
)

func TestCreateJob(t *testing.T) {
	conf := option.Config{
		EtcdEndPoints:       []string{"192.168.2.203:2379"},
		MQAPI:               "192.168.2.203:6300",
		EventLogServers:     []string{"192.168.2.203:6366"},
		RbdRepoName:         "rbd-dns",
		RbdNamespace:        "rbd-system",
		MysqlConnectionInfo: "EeM2oc:lee7OhQu@tcp(192.168.2.203:3306)/region",
	}
	event.NewManager(event.EventConfig{
		EventLogServers: conf.EventLogServers,
		DiscoverArgs:    &etcdutil.ClientArgs{Endpoints: conf.EtcdEndPoints},
	})
	restConfig, err := k8sutil.NewRestConfig("/Users/fanyangyang/Documents/company/goodrain/remote/192.168.2.206/admin.kubeconfig")
	if err != nil {
		t.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatal(err)
	}
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Fatal("new docker error: ", err.Error())
	}
	logger := event.GetManager().GetLogger("0000")
	req := Request{
		ServerType:    "git",
		DockerClient:  dockerClient,
		KubeClient:    clientset,
		ServiceID:     "d9b8d718510dc53118af1e1219e36d3a",
		DeployVersion: "123",
		TenantID:      "7c89455140284fd7b263038b44dc65bc",
		Lang:          code.JavaMaven,
		Runtime:       "1.8",
		Logger:        logger,
	}
	req.BuildEnvs = map[string]string{
		"PROCFILE": "web: java $JAVA_OPTS -jar target/java-maven-demo-0.0.1.jar",
		"PROC_ENV": `{"procfile": "", "dependencies": {}, "language": "Java-maven", "runtimes": "1.8"}`,
		"RUNTIME":  "1.8",
	}
	req.CacheDir = fmt.Sprintf("/cache/build/%s/cache/%s", req.TenantID, req.ServiceID)
	req.TGZDir = fmt.Sprintf("/grdata/build/tenant/%s/slug/%s", req.TenantID, req.ServiceID)
	req.SourceDir = fmt.Sprintf("/cache/source/build/%s/%s", req.TenantID, req.ServiceID)
	sb := slugBuild{tgzDir: "string"}
	if err := sb.runBuildJob(&req); err != nil {
		t.Fatal(err)
	}
	fmt.Println("create job finished")

}

func Test1(t *testing.T) {
	tarFile := "/opt/rainbond/pkg/rainbond-pkg-V5.2-dev.tgz"
	srcFile, err := os.Open(tarFile)
	if err != nil {
		t.Fatal(err)
	}
	defer srcFile.Close()
	gr, err := gzip.NewReader(srcFile) //handle gzip feature
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr) // tar reader
	now := time.Now()
	for hdr, err := tr.Next(); err != io.EOF; hdr, err = tr.Next() { // next range tar info
		if err != nil {
			t.Fatal(err)
			continue
		}
		// 读取文件信息
		fi := hdr.FileInfo()

		if !strings.HasPrefix(fi.Name(), "._") && strings.HasSuffix(fi.Name(), ".tgz") {
			t.Logf("name: %s, size: %d", fi.Name(), fi.Size())

		}
	}
	t.Logf("cost: %d", time.Since(now))
}

func TestDockerClient(t *testing.T) {
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Fatal("new docker error: ", err.Error())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, container := range containers {
		t.Log("container id : ", container.ID)
	}
	// images, err := dockerClient.ImageList(ctx, types.ImageListOptions{})
	// for _, image := range images {
	// 	t.Log("image is : ", image.ID)
	// }
}
