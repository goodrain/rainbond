package discover

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/discover/config"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

func TestK8sDiscover_AddProject(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "ok",
		},
	}

	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := &option.Conf{RbdNamespace: ""}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rbd-gateway-abcde",
					Labels: map[string]string{
						"name": "rbd-gateway",
					},
				},
				Status: corev1.PodStatus{
					PodIP: "172.20.0.20",
				},
			}
			clientset := fake.NewSimpleClientset(pod)

			discover := NewK8sDiscover(ctx, clientset, cfg)
			defer discover.Stop()

			callback := &testCallback{
				epCh:  make(chan []*config.Endpoint),
				errCh: make(chan error),
			}
			discover.AddProject("rbd-gateway", callback)

			go func() {
				for {
					select {
					case endpoints := <-callback.epCh:
						for _, ep := range endpoints {
							fmt.Printf("%#v\n", ep)
						}
					case err := <-callback.errCh:
						t.Errorf("received unexpected error from callback: %v", err)
						return
					default:

					}
				}
			}()

			time.Sleep(2 * time.Second)

			pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			})
			pod.Status.PodIP = "172.20.0.50"
			_, err := clientset.CoreV1().Pods("").Update(pod)
			if err != nil {
				t.Error(err)
			}

			time.Sleep(1 * time.Second)

			err = clientset.CoreV1().Pods("").Delete(pod.Name, &metav1.DeleteOptions{})
			if err != nil {
				t.Error(err)
			}

			time.Sleep(30 * time.Second)
		})
	}
}

type testCallback struct {
	epCh  chan []*config.Endpoint
	errCh chan error
}

func (t *testCallback) UpdateEndpoints(endpoints ...*config.Endpoint) {
	t.epCh <- endpoints
}

func (t *testCallback) Error(err error) {
	fmt.Println(err)
}
