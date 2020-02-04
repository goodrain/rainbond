package build

import (
	"archive/tar"
	"bufio"
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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func TestGetPogLog(t *testing.T) {
	restConfig, err := k8sutil.NewRestConfig("/Users/fanyangyang/Documents/company/goodrain/remote/192.168.2.206/admin.kubeconfig")
	if err != nil {
		t.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatal(err)
	}
	re := Request{
		KubeClient: clientset,
		ServiceID:  "aae30a8d6a66ea9024197bc8deecd137",
	}

	for {
		fmt.Println("waiting job finish")
		time.Sleep(5 * time.Second)
		job, err := re.KubeClient.BatchV1().Jobs("rbd-system").Get(re.ServiceID, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("get job error: %s", err.Error())
		}
		if job == nil {
			continue
		}
		if job.Status.Active > 0 {
			fmt.Println("build job start")
			var po corev1.Pod
			labelSelector := fmt.Sprintf("job-name=%s", re.ServiceID)
			for {
				pos, err := re.KubeClient.CoreV1().Pods("rbd-system").List(metav1.ListOptions{LabelSelector: labelSelector})
				if err != nil {
					fmt.Printf(" get po error: %s", err.Error())
				}
				if len(pos.Items) == 0 {
					continue
				}
				if len(pos.Items[0].Spec.Containers) > 0 {
					fmt.Println("pod container ready, start write log")
					po = pos.Items[0]
					break
				}
				time.Sleep(5 * time.Second)
			}
			podLogRequest := re.KubeClient.CoreV1().Pods("rbd-system").GetLogs(po.Name, &corev1.PodLogOptions{Follow: true})
			reader, err := podLogRequest.Stream()
			if err != nil {
				fmt.Println("get build job pod log data error: ", err.Error())
				continue
			}
			defer reader.Close()
			bufReader := bufio.NewReader(reader)
			for {
				line, err := bufReader.ReadBytes('\n')
				fmt.Println(string(line))
				if err == io.EOF {
					break
				}
				if err != nil {
					fmt.Printf("get job log error: %s", err.Error())
					break
				}

			}
		}
		if job.Status.Succeeded > 0 {
			fmt.Println("build job have done successfully")
			if err = re.KubeClient.BatchV1().Jobs("rbd-system").Delete(re.ServiceID, &metav1.DeleteOptions{}); err != nil {
				fmt.Printf("delete job failed: %s", err.Error())
			}
			break
		}
		if job.Status.Failed > 0 {
			fmt.Println("build job have done failed")
			if err = re.KubeClient.BatchV1().Jobs("rbd-system").Delete(re.ServiceID, &metav1.DeleteOptions{}); err != nil {
				fmt.Printf("delete job failed: %s", err.Error())
			}
			break
		}
	}
}

func TestDeleteJobAuto(t *testing.T) {
	restConfig, err := k8sutil.NewRestConfig("/Users/fanyangyang/Documents/company/goodrain/remote/192.168.2.206/admin.kubeconfig")
	if err != nil {
		t.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatal(err)
	}
	job := batchv1.Job{}
	job.Name = "fanyangyang"
	job.Namespace = "rbd-system"

	var ttl int32
	ttl = 0
	job.Spec.TTLSecondsAfterFinished = &ttl //  k8s version >= 1.12
	job.Spec = batchv1.JobSpec{
		TTLSecondsAfterFinished: &ttl,
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					corev1.Container{
						Name:    "fanyangyang",
						Image:   "busybox",
						Command: []string{"echo", "hello job"},
					},
				},
			},
		},
	}

	_, err = clientset.BatchV1().Jobs(job.Namespace).Create(&job)
	if err != nil {
		t.Fatal("create job error: ", err.Error())
	}

	for {
		j, err := clientset.BatchV1().Jobs(job.Namespace).Get(job.Name, metav1.GetOptions{})
		if err != nil {
			t.Error("get job error: ", err.Error())
		}
		if j == nil {
			continue
		}
		if j.Status.Active > 0 {
			fmt.Println("job is running")
		}
		if j.Status.Succeeded > 0 {
			fmt.Println("job is succeed, waiting auto delete")
			break
		}
		if j.Status.Failed > 0 {
			fmt.Println("job is failed, waiting next")
			break
		}
		time.Sleep(5 * time.Second)
	}
}

func TestDeleteJob(t *testing.T) {
	podChan := make(chan struct{})
	defer close(podChan)
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
	name := "fanyangyang"
	namespace := "rbd-system"
	logger := event.GetManager().GetLogger("0000")
	writer := logger.GetWriter("builder", "info")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go getJobPodLogs(ctx, podChan, clientset, writer, namespace, name)
	getJob(ctx, podChan, clientset, namespace, name)
	t.Log("done")
}
