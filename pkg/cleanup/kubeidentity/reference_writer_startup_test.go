package kubeidentity

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestReferenceWriterStartupWaitsForOwnRuntimeOnly(t *testing.T) {
	for _, replaced := range []bool{false, true} {
		t.Run(map[bool]string{false: "delayed", true: "replaced"}[replaced], func(t *testing.T) {
			database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "startup.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			database.LogMode(false)
			if err := database.AutoMigrate(&model.CleanupReferenceWriter{}).Error; err != nil {
				t.Fatal(err)
			}
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "system", UID: "original", ResourceVersion: "1", Labels: map[string]string{"name": "rbd-api"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "rbd-api"}}}, Status: corev1.PodStatus{Phase: corev1.PodPending}}
			client := fake.NewSimpleClientset()
			calls := 0
			client.PrependReactor("get", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
				calls++
				current := pod.DeepCopy()
				if calls >= 3 {
					current.ResourceVersion = "2"
					current.Status.Phase = corev1.PodRunning
					current.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "rbd-api", ContainerID: "containerd://running", ImageID: "immutable-image", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
					if replaced {
						current.UID = "replacement"
					}
				}
				return true, current, nil
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err = RegisterReferenceWriterStartup(ctx, database, client, "system", "api", "api")
			var count int
			database.Model(&model.CleanupReferenceWriter{}).Count(&count)
			if replaced {
				if err == nil || count != 0 {
					t.Fatal("replacement inherited startup", err, count)
				}
			} else {
				if err != nil || count != 2 {
					t.Fatal(err, count)
				}
				observed, err := InspectReferenceWriter(ctx, client, "system", "api", "original", "api")
				if err != nil {
					t.Fatal(err)
				}
				if known, err := guard.ReferenceWriterRegistered(database, observed); err != nil || !known {
					t.Fatal(known, err)
				}
				observed.Protocol = guard.UploadWriterProtocol
				if known, err := guard.ReferenceWriterRegistered(database, observed); err != nil || !known {
					t.Fatal("missing actual upload writer startup", err)
				}
			}
		})
	}
}
