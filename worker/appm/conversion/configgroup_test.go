package conversion

import (
	corev1 "k8s.io/api/core/v1"
	"testing"
)

// capability_id: rainbond.worker.config-group-component-isolation
func TestConfigGroupComponentIsolation(t *testing.T) {
	const template = "-Darms.appName=${_NAMESPACE}-${_SERVICE_NAME}"
	shared := &corev1.Secret{Data: map[string][]byte{
		"JAVA_TOOL_OPTIONS": []byte(template), "OVERRIDE": []byte("group"), "STATIC": []byte("unchanged"),
	}}
	for _, name := range []string{"gateway", "system", "gateway"} {
		envs := []corev1.EnvVar{{Name: "_NAMESPACE", Value: "prod"}, {Name: "_SERVICE_NAME", Value: name}, {Name: "OVERRIDE", Value: "component"}}
		config := map[string]string{"_NAMESPACE": "prod", "_SERVICE_NAME": name, "JAVA_TOOL_OPTIONS": template, "OVERRIDE": "component", "STATIC": "unchanged"}
		got := appendConfigGroupEnvs(envs, []*corev1.Secret{shared}, config)
		values := map[string]string{}
		for _, env := range got {
			if _, exists := values[env.Name]; exists {
				t.Fatalf("duplicate env %s", env.Name)
			}
			values[env.Name] = env.Value
		}
		if values["JAVA_TOOL_OPTIONS"] != "-Darms.appName=prod-"+name {
			t.Fatalf("%s received %q", name, values["JAVA_TOOL_OPTIONS"])
		}
		if values["OVERRIDE"] != "component" {
			t.Fatal("component override lost")
		}
		if values["STATIC"] != "unchanged" {
			t.Fatal("static configuration lost")
		}
		if string(shared.Data["JAVA_TOOL_OPTIONS"]) != template {
			t.Fatal("shared template was mutated")
		}
	}
	if got := appendConfigGroupEnvs(nil, nil, nil); len(got) != 0 {
		t.Fatal("unexpected environment without groups")
	}
}
