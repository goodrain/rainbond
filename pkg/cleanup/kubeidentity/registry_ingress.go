package kubeidentity

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// RegistryIngressObservation reports only verified port routing. It does not
// prove that a Registry has no background writers or that references are complete.
type RegistryIngressObservation struct{ RegistryPort, ProxyPort int32 }

func literalEnv(container *corev1.Container, name string) (string, error) {
	found := false
	value := ""
	for _, env := range container.Env {
		if env.Name == name {
			if found || env.ValueFrom != nil {
				return "", ErrBinding
			}
			found = true
			value = env.Value
		}
	}
	if !found {
		return "", ErrBinding
	}
	return value, nil
}
func explicitArguments(container *corev1.Container) (map[string]string, error) {
	result := map[string]string{}
	// Installer-generated arguments use the unambiguous --name=value form.
	// Reject duplicates rather than disagree with flag parsing's last-value rule.
	for _, arg := range container.Args {
		if !strings.HasPrefix(arg, "--") {
			return nil, ErrBinding
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		if !ok || key == "" {
			return nil, ErrBinding
		}
		if _, exists := result[key]; exists {
			return nil, ErrBinding
		}
		result[key] = value
	}
	if value := result["initialize-storage-identity"]; value != "" && value != "false" {
		return nil, ErrBinding
	}
	return result, nil
}
func tcpAddress(value string) (string, int32, error) {
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return "", 0, ErrBinding
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", 0, ErrBinding
	}
	return host, int32(number), nil
}

// InspectRegistryIngress refuses host networking and any advertised path to the
// native Registry. The control plane supplies the owned Service and current Pod.
func InspectRegistryIngress(service *corev1.Service, pod *corev1.Pod, nativeName, sidecarName string) (RegistryIngressObservation, error) {
	denied := RegistryIngressObservation{}
	if service == nil || pod == nil || pod.Namespace != service.Namespace || pod.UID == "" || pod.DeletionTimestamp != nil || pod.Spec.HostNetwork || len(pod.Spec.Containers) != 2 || len(pod.Spec.EphemeralContainers) != 0 || nativeName == sidecarName || len(service.Spec.Selector) == 0 || !labels.SelectorFromSet(service.Spec.Selector).Matches(labels.Set(pod.Labels)) {
		return denied, ErrBinding
	}
	for _, init := range pod.Spec.InitContainers {
		finished := false
		for _, status := range pod.Status.InitContainerStatuses {
			if status.Name == init.Name && status.State.Terminated != nil && status.State.Terminated.ExitCode == 0 {
				finished = true
			}
		}
		if !finished {
			return denied, ErrBinding
		}
	}
	native, err := containerByName(pod, nativeName)
	if err != nil {
		return denied, err
	}
	sidecar, err := containerByName(pod, sidecarName)
	if err != nil {
		return denied, err
	}
	address, err := literalEnv(native, "REGISTRY_HTTP_ADDR")
	if err != nil {
		return denied, err
	}
	host, nativePort, err := tcpAddress(address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return denied, ErrBinding
	}
	for _, port := range native.Ports {
		if port.HostPort != 0 {
			return denied, ErrBinding
		}
	}
	if len(sidecar.Command) != 1 || sidecar.Command[0] != "/registry-coordinator" {
		return denied, ErrBinding
	}
	args, err := explicitArguments(sidecar)
	if err != nil {
		return denied, err
	}
	publicHost, proxyPort, err := tcpAddress(args["listen"])
	if err != nil || (publicHost != "" && publicHost != "0.0.0.0" && publicHost != "::") || proxyPort == nativePort {
		return denied, ErrBinding
	}
	upstream, err := url.Parse(args["upstream"])
	if err != nil || upstream.User != nil || upstream.RawQuery != "" || upstream.Fragment != "" || (upstream.Path != "" && upstream.Path != "/") || (upstream.Scheme != "http" && upstream.Scheme != "https") {
		return denied, ErrBinding
	}
	upstreamHost, upstreamPort, err := tcpAddress(upstream.Host)
	if err != nil || upstreamPort != nativePort || net.ParseIP(upstreamHost) == nil || !net.ParseIP(upstreamHost).Equal(net.ParseIP(host)) {
		return denied, ErrBinding
	}
	ports := map[string]int32{}
	nativeNames := map[string]bool{}
	for _, port := range native.Ports {
		if port.Name != "" {
			nativeNames[port.Name] = true
		}
	}
	for _, port := range sidecar.Ports {
		if port.HostPort != 0 && port.ContainerPort != proxyPort {
			return denied, ErrBinding
		}
		if port.Name != "" {
			if nativeNames[port.Name] {
				return denied, ErrBinding
			}
			if _, exists := ports[port.Name]; exists {
				return denied, ErrBinding
			}
			if port.Protocol != "" && port.Protocol != corev1.ProtocolTCP {
				return denied, ErrBinding
			}
			ports[port.Name] = port.ContainerPort
		}
	}
	if len(service.Spec.Ports) == 0 || service.Spec.Type == corev1.ServiceTypeExternalName {
		return denied, ErrBinding
	}
	for _, port := range service.Spec.Ports {
		if port.Protocol != "" && port.Protocol != corev1.ProtocolTCP {
			return denied, ErrBinding
		}
		target := port.TargetPort.IntVal
		if port.TargetPort.Type == intstr.String {
			target = ports[port.TargetPort.StrVal]
		}
		if target != proxyPort {
			return denied, ErrBinding
		}
	}
	return RegistryIngressObservation{RegistryPort: nativePort, ProxyPort: proxyPort}, nil
}
