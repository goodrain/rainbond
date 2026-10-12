package kubeidentity

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// capability_id: rainbond.cleanup.registry-ingress-isolation
func TestRegistryIngressRequiresExclusiveCoordinatedRoute(t *testing.T) {
	for _, scenario := range []string{"valid", "host-network", "native-public", "wrong-target", "wrong-upstream", "duplicate-listen", "extra-container", "duplicate-port-name", "active-init"} {
		t.Run(scenario, func(t *testing.T) {
			service, pod, _, _ := bindingObjects()
			service.Spec.Ports = []corev1.ServicePort{{Port: 5000, TargetPort: intstr.FromString("coordinated"), Protocol: corev1.ProtocolTCP}}
			pod.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "REGISTRY_HTTP_ADDR", Value: "127.0.0.1:5000"}}
			pod.Spec.Containers[1].Command = []string{"/registry-coordinator"}
			pod.Spec.Containers[1].Args = []string{"--listen=:5001", "--upstream=http://127.0.0.1:5000"}
			pod.Spec.Containers[1].Ports = []corev1.ContainerPort{{Name: "coordinated", ContainerPort: 5001, Protocol: corev1.ProtocolTCP}}
			switch scenario {
			case "host-network":
				pod.Spec.HostNetwork = true
			case "native-public":
				pod.Spec.Containers[0].Env[0].Value = ":5000"
			case "wrong-target":
				service.Spec.Ports[0].TargetPort = intstr.FromInt(5000)
			case "wrong-upstream":
				pod.Spec.Containers[1].Args[1] = "--upstream=http://127.0.0.1:5002"
			case "duplicate-listen":
				pod.Spec.Containers[1].Args = append(pod.Spec.Containers[1].Args, "--listen=:5002")
			case "duplicate-port-name":
				pod.Spec.Containers[0].Ports = []corev1.ContainerPort{{Name: "coordinated", ContainerPort: 5000}}
			case "active-init":
				pod.Spec.InitContainers = []corev1.Container{{Name: "extra-server"}}
			case "extra-container":
				pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: "unknown"})
			}
			observation, err := InspectRegistryIngress(service, pod, "registry", "coordinator")
			if scenario == "valid" {
				if err != nil || observation.ProxyPort != 5001 || observation.RegistryPort != 5000 {
					t.Fatal(observation, err)
				}
			} else if err == nil {
				t.Fatal("unverified ingress accepted")
			}
		})
	}
}
