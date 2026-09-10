package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	apisixversioned "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned"
	apisixfake "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned/fake"
	apimodel "github.com/goodrain/rainbond/api/model"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.gateway.jwt-consumer-management
func TestValidateGatewayJWTConsumerCredential(t *testing.T) {
	validHSSecret := strings.Repeat("s", gatewayJWTMinimumSecretBytes)
	validBase64Secret := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xfb}, gatewayJWTMinimumSecretBytes))
	tests := []struct {
		name       string
		credential *apimodel.GatewayJWTConsumerCredential
		wantErr    bool
	}{
		{
			name: "HS256 with an explicit secret",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:       "orders-client",
				Secret:    validHSSecret,
				Algorithm: gatewayJWTAlgorithmHS256,
			},
		},
		{
			name: "HS512 can request a generated secret",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:       "mobile-client",
				Algorithm: gatewayJWTAlgorithmHS512,
			},
		},
		{
			name: "RS256 with a public key",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:       "partner-client",
				Algorithm: gatewayJWTAlgorithmRS256,
				PublicKey: "test-rsa-public-key",
			},
		},
		{
			name: "default algorithm",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:    "default-client",
				Secret: validHSSecret,
			},
		},
		{
			name: "standard padded base64 HS secret",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:          "base64-client",
				Secret:       validBase64Secret,
				Algorithm:    gatewayJWTAlgorithmHS256,
				Base64Secret: true,
			},
		},
		{
			name: "missing key",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Secret:    validHSSecret,
				Algorithm: gatewayJWTAlgorithmHS256,
			},
			wantErr: true,
		},
		{
			name: "unsupported algorithm",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:       "orders-client",
				Secret:    validHSSecret,
				Algorithm: "none",
			},
			wantErr: true,
		},
		{
			name: "RS256 without a public key",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:       "partner-client",
				Algorithm: gatewayJWTAlgorithmRS256,
			},
			wantErr: true,
		},
		{
			name: "whitespace-only explicit HS secret",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:       "orders-client",
				Secret:    "   ",
				Algorithm: gatewayJWTAlgorithmHS256,
			},
			wantErr: true,
		},
		{
			name: "short explicit HS secret",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:       "orders-client",
				Secret:    strings.Repeat("s", gatewayJWTMinimumSecretBytes-1),
				Algorithm: gatewayJWTAlgorithmHS512,
			},
			wantErr: true,
		},
		{
			name: "invalid standard base64 HS secret",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:          "orders-client",
				Secret:       strings.Repeat("-", 44),
				Algorithm:    gatewayJWTAlgorithmHS256,
				Base64Secret: true,
			},
			wantErr: true,
		},
		{
			name: "non-canonical base64 HS secret containing a newline",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:          "orders-client",
				Secret:       validBase64Secret[:4] + "\n" + validBase64Secret[4:],
				Algorithm:    gatewayJWTAlgorithmHS256,
				Base64Secret: true,
			},
			wantErr: true,
		},
		{
			name: "decoded base64 HS secret is too short",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:          "orders-client",
				Secret:       base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("s"), gatewayJWTMinimumSecretBytes-1)),
				Algorithm:    gatewayJWTAlgorithmHS256,
				Base64Secret: true,
			},
			wantErr: true,
		},
		{
			name: "negative expiration",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:       "orders-client",
				Secret:    validHSSecret,
				Algorithm: gatewayJWTAlgorithmHS256,
				Exp:       -1,
			},
			wantErr: true,
		},
		{
			name: "negative lifetime grace period",
			credential: &apimodel.GatewayJWTConsumerCredential{
				Key:                 "orders-client",
				Secret:              validHSSecret,
				Algorithm:           gatewayJWTAlgorithmHS256,
				LifetimeGracePeriod: -1,
			},
			wantErr: true,
		},
	}

	if err := ValidateGatewayJWTConsumerCredential(nil); err == nil {
		t.Fatal("ValidateGatewayJWTConsumerCredential(nil) returned no error")
	}
	if err := ValidateGatewayJWTConsumerCredential(&apimodel.GatewayJWTConsumerCredential{
		Key:       "invalid-client",
		Algorithm: "none",
	}); !errors.Is(err, ErrGatewayJWTConsumerInvalidCredential) {
		t.Fatalf("invalid credential error = %v, want ErrGatewayJWTConsumerInvalidCredential", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGatewayJWTConsumerCredential(tt.credential)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateGatewayJWTConsumerCredential() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	t.Run("omitted HS secret is generated without mutating the request", func(t *testing.T) {
		credential := &apimodel.GatewayJWTConsumerCredential{
			Key:       "generated-client",
			Algorithm: gatewayJWTAlgorithmHS256,
		}

		data, generatedSecret, err := buildGatewayJWTSecretData(credential)
		if err != nil {
			t.Fatalf("buildGatewayJWTSecretData() error = %v", err)
		}
		if generatedSecret == "" {
			t.Fatal("buildGatewayJWTSecretData() did not return the generated secret")
		}
		if string(data[gatewayJWTSecretDataSecret]) != generatedSecret {
			t.Fatal("generated secret and Secret data differ")
		}
		if credential.Secret != "" {
			t.Fatal("buildGatewayJWTSecretData() mutated the request credential")
		}
		if strings.ContainsAny(generatedSecret, "+/=") {
			t.Fatalf("generated secret %q is not raw URL-safe base64", generatedSecret)
		}
		if string(data[gatewayJWTSecretDataExp]) != "86400" {
			t.Fatalf("default expiration = %q, want 86400", data[gatewayJWTSecretDataExp])
		}
	})

	t.Run("base64 mode generates standard padded base64", func(t *testing.T) {
		credential := &apimodel.GatewayJWTConsumerCredential{
			Key:          "generated-base64-client",
			Algorithm:    gatewayJWTAlgorithmHS256,
			Base64Secret: true,
		}

		data, generatedSecret, err := buildGatewayJWTSecretData(credential)
		if err != nil {
			t.Fatalf("buildGatewayJWTSecretData() error = %v", err)
		}
		decoded, err := base64.StdEncoding.Strict().DecodeString(generatedSecret)
		if err != nil {
			t.Fatalf("generated secret is not strict standard base64: %v", err)
		}
		if len(decoded) != gatewayJWTGeneratedSecretBytes {
			t.Fatalf("decoded generated secret length = %d", len(decoded))
		}
		if !strings.HasSuffix(generatedSecret, "=") || strings.ContainsAny(generatedSecret, "-_") {
			t.Fatalf("generated secret %q does not use padded standard base64", generatedSecret)
		}
		if string(data[gatewayJWTSecretDataSecret]) != generatedSecret {
			t.Fatal("generated base64 secret and Secret data differ")
		}
	})

	t.Run("explicit HS secret is never projected as generated", func(t *testing.T) {
		credential := &apimodel.GatewayJWTConsumerCredential{
			Key:       "explicit-client",
			Secret:    validHSSecret,
			Algorithm: gatewayJWTAlgorithmHS512,
		}

		data, generatedSecret, err := buildGatewayJWTSecretData(credential)
		if err != nil {
			t.Fatalf("buildGatewayJWTSecretData() error = %v", err)
		}
		if generatedSecret != "" {
			t.Fatal("caller-provided secret was exposed as a generated secret")
		}
		if string(data[gatewayJWTSecretDataSecret]) != credential.Secret {
			t.Fatal("explicit secret was not stored")
		}
	})

	t.Run("invalid credential is rejected before Secret data is built", func(t *testing.T) {
		data, generatedSecret, err := buildGatewayJWTSecretData(&apimodel.GatewayJWTConsumerCredential{})
		if err == nil {
			t.Fatal("buildGatewayJWTSecretData() returned no error")
		}
		if data != nil || generatedSecret != "" {
			t.Fatalf("invalid credential returned data=%#v generatedSecret=%q", data, generatedSecret)
		}
	})

	t.Run("RS256 data contains the APISIX translator keys", func(t *testing.T) {
		credential := &apimodel.GatewayJWTConsumerCredential{
			Key:                 "partner-client",
			Algorithm:           gatewayJWTAlgorithmRS256,
			PublicKey:           "test-public-key",
			PrivateKey:          "test-private-key",
			Exp:                 3600,
			LifetimeGracePeriod: 30,
		}

		data, generatedSecret, err := buildGatewayJWTSecretData(credential)
		if err != nil {
			t.Fatalf("buildGatewayJWTSecretData() error = %v", err)
		}
		if generatedSecret != "" {
			t.Fatal("RS256 unexpectedly generated a symmetric secret")
		}
		want := map[string][]byte{
			gatewayJWTSecretDataKey:                 []byte("partner-client"),
			gatewayJWTSecretDataSecret:              {},
			gatewayJWTSecretDataPublicKey:           []byte("test-public-key"),
			gatewayJWTSecretDataPrivateKey:          []byte("test-private-key"),
			gatewayJWTSecretDataAlgorithm:           []byte(gatewayJWTAlgorithmRS256),
			gatewayJWTSecretDataExp:                 []byte("3600"),
			gatewayJWTSecretDataBase64Secret:        []byte("false"),
			gatewayJWTSecretDataLifetimeGracePeriod: []byte("30"),
		}
		if !reflect.DeepEqual(data, want) {
			t.Fatalf("buildGatewayJWTSecretData() = %#v, want %#v", data, want)
		}
	})
}

func TestBuildManagedJWTPlugins(t *testing.T) {
	existing := []v2.ApisixRoutePlugin{
		{Name: "proxy-rewrite", Enable: true, Config: v2.ApisixRoutePluginConfig{"regex_uri": []interface{}{`^/api/(.*)`, "/$1"}}},
		{Name: gatewayJWTAuthPluginName, Enable: false, Config: v2.ApisixRoutePluginConfig{"header": "legacy-header"}, SecretRef: "legacy-ref"},
		{Name: "future-plugin", Enable: true, Config: v2.ApisixRoutePluginConfig{"mode": "keep"}},
		{Name: gatewayConsumerRestrictionPluginName, Enable: true, Config: v2.ApisixRoutePluginConfig{"whitelist": []string{"legacy_consumer"}}},
		{Name: gatewayJWTAuthPluginName, Enable: true, Config: v2.ApisixRoutePluginConfig{"query": "duplicate"}},
	}
	jwtConfig := v2.ApisixRoutePluginConfig{
		"header":           "authorization",
		"claims_to_verify": []interface{}{"exp", "nbf"},
	}

	got := BuildManagedJWTPlugins("team-a", []string{"orders", "mobile", "orders"}, jwtConfig, existing)

	want := []v2.ApisixRoutePlugin{
		existing[0],
		existing[2],
		{Name: gatewayJWTAuthPluginName, Enable: true, Config: jwtConfig},
		{
			Name:   gatewayConsumerRestrictionPluginName,
			Enable: true,
			Config: v2.ApisixRoutePluginConfig{
				"whitelist": []string{"team-a_orders", "team-a_mobile"},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildManagedJWTPlugins() = %#v, want %#v", got, want)
	}

	jwtConfig["header"] = "mutated"
	if got[2].Config["header"] != "authorization" {
		t.Fatal("BuildManagedJWTPlugins() retained an alias to the caller's config")
	}
	got[0].Config["new"] = "mutated"
	if existing[0].Config["new"] != nil {
		t.Fatal("BuildManagedJWTPlugins() retained an alias to an ordinary plugin config")
	}
}

func TestManagedJWTAuthenticationFromRoute(t *testing.T) {
	plugins := BuildManagedJWTPlugins(
		"team-a",
		[]string{"orders", "mobile"},
		v2.ApisixRoutePluginConfig{"header": "authorization", "query": "jwt"},
		nil,
	)
	t.Run("marked route is decoded", func(t *testing.T) {
		route := &v2.ApisixRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "orders-route",
				Namespace: "team-a",
				Labels: map[string]string{
					gatewayJWTManagedRouteLabel: gatewayJWTManagedRouteValue,
				},
			},
			Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{Plugins: plugins}}},
		}

		got := ManagedJWTAuthenticationFromRoute(route)
		want := &apimodel.ManagedJWTAuthentication{
			Enabled:       true,
			ConsumerNames: []string{"orders", "mobile"},
			Config:        v2.ApisixRoutePluginConfig{"header": "authorization", "query": "jwt"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ManagedJWTAuthenticationFromRoute() = %#v, want %#v", got, want)
		}
	})

	t.Run("legacy plugins without the marker are preserved but not inferred", func(t *testing.T) {
		route := &v2.ApisixRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "legacy-route", Namespace: "team-a"},
			Spec:       v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{Plugins: plugins}}},
		}

		if got := ManagedJWTAuthenticationFromRoute(route); got != nil {
			t.Fatalf("ManagedJWTAuthenticationFromRoute() = %#v, want nil", got)
		}
	})

	t.Run("marked route without HTTP rules remains safe", func(t *testing.T) {
		route := &v2.ApisixRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "team-a",
				Labels: map[string]string{
					gatewayJWTManagedRouteLabel: gatewayJWTManagedRouteValue,
				},
			},
		}

		if got := ManagedJWTAuthenticationFromRoute(route); got != nil {
			t.Fatalf("ManagedJWTAuthenticationFromRoute() = %#v, want nil", got)
		}
	})

	for _, test := range []struct {
		name    string
		plugins []v2.ApisixRoutePlugin
	}{
		{
			name: "missing consumer restriction is not reported as managed",
			plugins: []v2.ApisixRoutePlugin{
				{Name: gatewayJWTAuthPluginName, Enable: true, Config: v2.ApisixRoutePluginConfig{"header": "authorization"}},
			},
		},
		{
			name: "disabled JWT plugin is not reported as managed",
			plugins: []v2.ApisixRoutePlugin{
				{Name: gatewayJWTAuthPluginName, Enable: false, Config: v2.ApisixRoutePluginConfig{"header": "authorization"}},
				{Name: gatewayConsumerRestrictionPluginName, Enable: true, Config: v2.ApisixRoutePluginConfig{"whitelist": []string{"team-a_orders"}}},
			},
		},
		{
			name: "disabled restriction plugin is not reported as managed",
			plugins: []v2.ApisixRoutePlugin{
				{Name: gatewayJWTAuthPluginName, Enable: true, Config: v2.ApisixRoutePluginConfig{"header": "authorization"}},
				{Name: gatewayConsumerRestrictionPluginName, Enable: false, Config: v2.ApisixRoutePluginConfig{"whitelist": []string{"team-a_orders"}}},
			},
		},
		{
			name: "empty whitelist is not reported as managed",
			plugins: []v2.ApisixRoutePlugin{
				{Name: gatewayJWTAuthPluginName, Enable: true, Config: v2.ApisixRoutePluginConfig{"header": "authorization"}},
				{Name: gatewayConsumerRestrictionPluginName, Enable: true, Config: v2.ApisixRoutePluginConfig{"whitelist": []string{}}},
			},
		},
		{
			name: "anonymous consumer bypass is not reported as managed",
			plugins: []v2.ApisixRoutePlugin{
				{Name: gatewayJWTAuthPluginName, Enable: true, Config: v2.ApisixRoutePluginConfig{"anonymous_consumer": "guest"}},
				{Name: gatewayConsumerRestrictionPluginName, Enable: true, Config: v2.ApisixRoutePluginConfig{"whitelist": []string{"team-a_orders"}}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			route := &v2.ApisixRoute{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "team-a",
					Labels: map[string]string{
						gatewayJWTManagedRouteLabel: gatewayJWTManagedRouteValue,
					},
				},
				Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{Plugins: test.plugins}}},
			}
			if got := ManagedJWTAuthenticationFromRoute(route); got != nil {
				t.Fatalf("ManagedJWTAuthenticationFromRoute() = %#v, want nil", got)
			}
		})
	}

	t.Run("JSON-decoded whitelist ignores malformed and foreign usernames", func(t *testing.T) {
		route := &v2.ApisixRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "team-a",
				Labels: map[string]string{
					gatewayJWTManagedRouteLabel: gatewayJWTManagedRouteValue,
				},
			},
			Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{Plugins: []v2.ApisixRoutePlugin{
				{
					Name:   gatewayJWTAuthPluginName,
					Enable: true,
					Config: v2.ApisixRoutePluginConfig{"header": "authorization"},
				},
				{
					Name:   gatewayConsumerRestrictionPluginName,
					Enable: true,
					Config: v2.ApisixRoutePluginConfig{
						"whitelist": []interface{}{"team-a_orders", "other_mobile", 42, "team-a_orders", "team-a_"},
					},
				},
			}}}},
		}

		got := ManagedJWTAuthenticationFromRoute(route)
		if !reflect.DeepEqual(got.ConsumerNames, []string{"orders"}) {
			t.Fatalf("consumer names = %#v, want orders", got.ConsumerNames)
		}
	})

	t.Run("safe response projection excludes credential material", func(t *testing.T) {
		consumer := &v2.ApisixConsumer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "orders",
				Namespace: "team-a",
				Labels: map[string]string{
					gatewayJWTManagedByLabel: gatewayJWTManagedValue,
					gatewayJWTAuthTypeLabel:  gatewayJWTAuthTypeValue,
				},
			},
			Spec: v2.ApisixConsumerSpec{AuthParameter: v2.ApisixConsumerAuthParameter{
				JwtAuth: &v2.ApisixConsumerJwtAuth{
					SecretRef: &corev1.LocalObjectReference{Name: "rbd-jwt-orders"},
				},
			}},
		}
		secret := managedGatewayJWTSecret("rbd-jwt-orders", "team-a", map[string][]byte{
			gatewayJWTSecretDataKey:        []byte("orders-client"),
			gatewayJWTSecretDataSecret:     []byte("must-not-leak-secret"),
			gatewayJWTSecretDataPublicKey:  []byte("must-not-leak-public-key"),
			gatewayJWTSecretDataPrivateKey: []byte("must-not-leak-private-key"),
			gatewayJWTSecretDataAlgorithm:  []byte(gatewayJWTAlgorithmRS256),
		})

		response := gatewayJWTConsumerResponse(consumer, secret, []string{"route-b", "route-a"})
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}
		for _, forbidden := range []string{"must-not-leak-secret", "must-not-leak-public-key", "must-not-leak-private-key"} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf("safe response leaked %q: %s", forbidden, encoded)
			}
		}
		if response.Username != "team-a_orders" {
			t.Fatalf("response username = %q, want team-a_orders", response.Username)
		}
		if response.Key != "orders-client" || response.Algorithm != gatewayJWTAlgorithmRS256 {
			t.Fatalf("safe metadata was not projected: %#v", response)
		}
		if response.Source != gatewayJWTManagedValue {
			t.Fatalf("response source = %q, want %q", response.Source, gatewayJWTManagedValue)
		}
		if !reflect.DeepEqual(response.BoundRoutes, []string{"route-a", "route-b"}) {
			t.Fatalf("response bound routes = %#v", response.BoundRoutes)
		}
	})

	t.Run("external response without a Secret remains safe", func(t *testing.T) {
		consumer := &v2.ApisixConsumer{
			ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: "team-a"},
			Status: v2.ApisixStatus{Conditions: []metav1.Condition{
				{Type: "ResourcesAvailable", Status: metav1.ConditionTrue},
			}},
		}

		response := gatewayJWTConsumerResponse(consumer, nil, nil)
		if response.Source != gatewayJWTExternalSource || response.Status != "ready" {
			t.Fatalf("external response = %#v", response)
		}
		if response.Key != "" || response.Algorithm != "" {
			t.Fatalf("external response unexpectedly contains credential metadata: %#v", response)
		}
		if response.BoundRoutes == nil {
			t.Fatal("external response encoded an empty route list as null")
		}
	})

	t.Run("consumer status reports errors and pending reconciliation", func(t *testing.T) {
		errorStatus := gatewayJWTConsumerStatus(v2.ApisixStatus{Conditions: []metav1.Condition{
			{Type: "Other", Status: metav1.ConditionTrue},
			{Type: "ResourcesAvailable", Status: metav1.ConditionFalse},
		}})
		if errorStatus != "error" {
			t.Fatalf("error status = %q", errorStatus)
		}

		unknownStatus := gatewayJWTConsumerStatus(v2.ApisixStatus{Conditions: []metav1.Condition{
			{Type: "ResourcesAvailable", Status: metav1.ConditionUnknown},
		}})
		if unknownStatus != gatewayJWTConsumerStatusPending {
			t.Fatalf("unknown status = %q", unknownStatus)
		}
	})

	if response := gatewayJWTConsumerResponse(nil, nil, nil); response != nil {
		t.Fatalf("gatewayJWTConsumerResponse(nil) = %#v, want nil", response)
	}
	if config := cloneApisixRoutePluginConfig(nil); config != nil {
		t.Fatalf("cloneApisixRoutePluginConfig(nil) = %#v, want nil", config)
	}
}

func gatewayActionForJWTConsumerTest(kubeClient kubernetes.Interface, apisixClient apisixversioned.Interface) *GatewayAction {
	return &GatewayAction{kubeClient: kubeClient, apisixClient: apisixClient}
}

func managedGatewayJWTLabels(appID string) map[string]string {
	labels := map[string]string{
		gatewayJWTManagedByLabel: gatewayJWTManagedValue,
		gatewayJWTAuthTypeLabel:  gatewayJWTAuthTypeValue,
	}
	if appID != "" {
		labels["app_id"] = appID
	}
	return labels
}

func managedGatewayJWTConsumer(name, namespace, secretName string) *v2.ApisixConsumer {
	return &v2.ApisixConsumer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    managedGatewayJWTLabels(""),
		},
		Spec: v2.ApisixConsumerSpec{
			IngressClassName: "apisix",
			AuthParameter: v2.ApisixConsumerAuthParameter{
				JwtAuth: &v2.ApisixConsumerJwtAuth{
					SecretRef: &corev1.LocalObjectReference{Name: secretName},
				},
			},
		},
	}
}

func managedGatewayJWTSecret(name, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    managedGatewayJWTLabels(""),
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}
}

func gatewayJWTDeleteUID(t *testing.T, action k8stesting.Action) types.UID {
	t.Helper()
	deleteAction, ok := action.(k8stesting.DeleteAction)
	if !ok {
		t.Fatalf("action %T is not a DeleteAction", action)
	}
	preconditions := deleteAction.GetDeleteOptions().Preconditions
	if preconditions == nil || preconditions.UID == nil {
		t.Fatal("delete action has no UID precondition")
	}
	return *preconditions.UID
}

func TestGatewayActionCreateJWTConsumer(t *testing.T) {
	const (
		name       = "orders"
		namespace  = "team-a"
		appID      = "app-1"
		secretName = "rbd-jwt-orders"
	)
	kubeClient := k8sfake.NewSimpleClientset()
	apisixClient := apisixfake.NewSimpleClientset()
	var creationOrder []string
	kubeClient.PrependReactor("create", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		creationOrder = append(creationOrder, "Secret")
		return false, nil, nil
	})
	apisixClient.PrependReactor("create", "apisixconsumers", func(k8stesting.Action) (bool, runtime.Object, error) {
		creationOrder = append(creationOrder, "ApisixConsumer")
		return false, nil, nil
	})
	action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)
	req := &apimodel.GatewayJWTConsumerRequest{
		Name: name,
		GatewayJWTConsumerCredential: apimodel.GatewayJWTConsumerCredential{
			Key:       "orders-client",
			Secret:    strings.Repeat("s", gatewayJWTMinimumSecretBytes),
			Algorithm: gatewayJWTAlgorithmHS512,
		},
	}

	got, err := action.CreateGatewayJWTConsumer(context.Background(), namespace, appID, req)
	if err != nil {
		t.Fatalf("CreateGatewayJWTConsumer() error = %v", err)
	}
	if !reflect.DeepEqual(creationOrder, []string{"Secret", "ApisixConsumer"}) {
		t.Fatalf("creation order = %#v, want Secret then ApisixConsumer", creationOrder)
	}
	if got.Name != name || got.Username != "team-a_orders" || got.Source != gatewayJWTManagedValue {
		t.Fatalf("CreateGatewayJWTConsumer() = %#v", got)
	}
	if got.GeneratedSecret != "" {
		t.Fatal("caller-provided Secret was returned as generated")
	}

	secret, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get created Secret: %v", err)
	}
	if secret.Type != corev1.SecretTypeOpaque {
		t.Fatalf("Secret type = %q, want Opaque", secret.Type)
	}
	if string(secret.Data[gatewayJWTSecretDataKey]) != "orders-client" ||
		string(secret.Data[gatewayJWTSecretDataSecret]) != req.Secret ||
		string(secret.Data[gatewayJWTSecretDataAlgorithm]) != gatewayJWTAlgorithmHS512 {
		t.Fatalf("created Secret data did not contain the requested credential metadata")
	}
	if !isManagedGatewayJWTResource(secret.Labels) || secret.Labels["app_id"] != appID {
		t.Fatalf("created Secret labels = %#v", secret.Labels)
	}

	consumer, err := apisixClient.ApisixV2().ApisixConsumers(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get created ApisixConsumer: %v", err)
	}
	if !isManagedGatewayJWTResource(consumer.Labels) || consumer.Labels["app_id"] != appID {
		t.Fatalf("created ApisixConsumer labels = %#v", consumer.Labels)
	}
	if consumer.Spec.IngressClassName != "apisix" || consumer.Spec.AuthParameter.JwtAuth == nil ||
		consumer.Spec.AuthParameter.JwtAuth.SecretRef == nil ||
		consumer.Spec.AuthParameter.JwtAuth.SecretRef.Name != secretName ||
		consumer.Spec.AuthParameter.JwtAuth.Value != nil {
		t.Fatalf("created ApisixConsumer auth parameter = %#v", consumer.Spec.AuthParameter.JwtAuth)
	}

	t.Run("generated secret is returned once", func(t *testing.T) {
		generated, err := action.CreateGatewayJWTConsumer(context.Background(), namespace, "", &apimodel.GatewayJWTConsumerRequest{
			Name: "mobile",
			GatewayJWTConsumerCredential: apimodel.GatewayJWTConsumerCredential{
				Key:       "mobile-client",
				Algorithm: gatewayJWTAlgorithmHS256,
			},
		})
		if err != nil {
			t.Fatalf("create with generated secret: %v", err)
		}
		if generated.GeneratedSecret == "" {
			t.Fatal("generated secret was not returned on create")
		}
		stored, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), "rbd-jwt-mobile", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get generated Secret: %v", err)
		}
		if string(stored.Data[gatewayJWTSecretDataSecret]) != generated.GeneratedSecret {
			t.Fatal("generated response and persisted Secret differ")
		}
		listed, err := action.ListGatewayJWTConsumers(context.Background(), namespace, "")
		if err != nil {
			t.Fatalf("list consumers: %v", err)
		}
		for _, item := range listed {
			if item.GeneratedSecret != "" {
				t.Fatalf("list exposed generated secret for %s", item.Name)
			}
		}
	})

	t.Run("invalid resource name is rejected before client calls", func(t *testing.T) {
		invalidKubeClient := k8sfake.NewSimpleClientset()
		invalidApisixClient := apisixfake.NewSimpleClientset()
		invalidAction := gatewayActionForJWTConsumerTest(invalidKubeClient, invalidApisixClient)
		_, err := invalidAction.CreateGatewayJWTConsumer(context.Background(), namespace, "", &apimodel.GatewayJWTConsumerRequest{
			Name: "INVALID_NAME",
			GatewayJWTConsumerCredential: apimodel.GatewayJWTConsumerCredential{
				Key: "invalid-client",
			},
		})
		if !errors.Is(err, ErrGatewayJWTConsumerInvalidName) {
			t.Fatalf("invalid name error = %v", err)
		}
		if len(invalidKubeClient.Actions()) != 0 || len(invalidApisixClient.Actions()) != 0 {
			t.Fatal("invalid request reached Kubernetes clients")
		}
	})
}

func TestGatewayActionCreateJWTConsumerRollsBackSecret(t *testing.T) {
	const (
		name       = "orders"
		namespace  = "team-a"
		secretName = "rbd-jwt-orders"
	)
	kubeClient := k8sfake.NewSimpleClientset()
	apisixClient := apisixfake.NewSimpleClientset()
	apisixClient.PrependReactor("create", "apisixconsumers", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("apisix consumer create failed")
	})
	action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)

	_, err := action.CreateGatewayJWTConsumer(context.Background(), namespace, "", &apimodel.GatewayJWTConsumerRequest{
		Name: name,
		GatewayJWTConsumerCredential: apimodel.GatewayJWTConsumerCredential{
			Key:       "orders-client",
			Secret:    strings.Repeat("s", gatewayJWTMinimumSecretBytes),
			Algorithm: gatewayJWTAlgorithmHS256,
		},
	})
	if err == nil {
		t.Fatal("CreateGatewayJWTConsumer() returned no error")
	}
	_, err = kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if !k8serrors.IsNotFound(err) {
		t.Fatalf("created Secret was not rolled back, get error = %v", err)
	}
	var secretDeletes int
	for _, clientAction := range kubeClient.Actions() {
		if clientAction.GetVerb() == "delete" && clientAction.GetResource().Resource == "secrets" {
			secretDeletes++
		}
	}
	if secretDeletes != 1 {
		t.Fatalf("Secret rollback delete count = %d, want 1", secretDeletes)
	}
}

func TestGatewayActionCreateJWTConsumerRollbackIgnoresPreexistingConsumerReference(t *testing.T) {
	const (
		name       = "orders"
		namespace  = "team-a"
		secretName = "rbd-jwt-orders"
	)
	createdUID := types.UID("created-by-this-request")
	preexisting := &v2.ApisixConsumer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v2.ApisixConsumerSpec{AuthParameter: v2.ApisixConsumerAuthParameter{
			JwtAuth: &v2.ApisixConsumerJwtAuth{
				SecretRef: &corev1.LocalObjectReference{Name: secretName},
			},
		}},
	}
	kubeClient := k8sfake.NewSimpleClientset()
	kubeClient.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		action.(k8stesting.CreateAction).GetObject().(*corev1.Secret).UID = createdUID
		return false, nil, nil
	})
	kubeClient.PrependReactor("delete", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if uid := gatewayJWTDeleteUID(t, action); uid != createdUID {
			t.Fatalf("rollback Secret UID = %q, want %q", uid, createdUID)
		}
		return false, nil, nil
	})
	action := gatewayActionForJWTConsumerTest(kubeClient, apisixfake.NewSimpleClientset(preexisting))

	_, err := action.CreateGatewayJWTConsumer(context.Background(), namespace, "", &apimodel.GatewayJWTConsumerRequest{
		Name: name,
		GatewayJWTConsumerCredential: apimodel.GatewayJWTConsumerCredential{
			Key:       "new-orders-client",
			Secret:    strings.Repeat("n", gatewayJWTMinimumSecretBytes),
			Algorithm: gatewayJWTAlgorithmHS256,
		},
	})
	if !k8serrors.IsAlreadyExists(err) {
		t.Fatalf("create error = %v, want existing Consumer conflict", err)
	}
	if _, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{}); !k8serrors.IsNotFound(err) {
		t.Fatalf("request-created Secret remained available to the pre-existing Consumer: %v", err)
	}
}

func TestGatewayActionCreateJWTConsumerRollbackPreservesReplacement(t *testing.T) {
	const (
		name       = "orders"
		namespace  = "team-a"
		secretName = "rbd-jwt-orders"
	)
	createdUID := types.UID("created-secret-uid")
	replacementUID := types.UID("replacement-secret-uid")
	kubeClient := k8sfake.NewSimpleClientset()
	apisixClient := apisixfake.NewSimpleClientset()
	kubeClient.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		action.(k8stesting.CreateAction).GetObject().(*corev1.Secret).UID = createdUID
		return false, nil, nil
	})
	apisixClient.PrependReactor("create", "apisixconsumers", func(k8stesting.Action) (bool, runtime.Object, error) {
		stored, err := kubeClient.Tracker().Get(corev1.SchemeGroupVersion.WithResource("secrets"), namespace, secretName)
		if err != nil {
			t.Fatalf("get created Secret before replacement: %v", err)
		}
		replacement := stored.(*corev1.Secret).DeepCopy()
		replacement.UID = replacementUID
		if err := kubeClient.Tracker().Update(corev1.SchemeGroupVersion.WithResource("secrets"), replacement, namespace); err != nil {
			t.Fatalf("replace Secret before rollback: %v", err)
		}
		return true, nil, errors.New("APISIX Consumer create failed")
	})
	kubeClient.PrependReactor("delete", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if uid := gatewayJWTDeleteUID(t, action); uid != createdUID {
			t.Fatalf("rollback Secret UID = %q, want %q", uid, createdUID)
		}
		return true, nil, k8serrors.NewConflict(corev1.Resource("secrets"), secretName, errors.New("UID precondition failed"))
	})
	action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)

	_, err := action.CreateGatewayJWTConsumer(context.Background(), namespace, "", &apimodel.GatewayJWTConsumerRequest{
		Name: name,
		GatewayJWTConsumerCredential: apimodel.GatewayJWTConsumerCredential{
			Key:       "orders-client",
			Secret:    strings.Repeat("s", gatewayJWTMinimumSecretBytes),
			Algorithm: gatewayJWTAlgorithmHS256,
		},
	})
	if !k8serrors.IsConflict(err) {
		t.Fatalf("rollback replacement error = %v, want Conflict", err)
	}
	stored, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("replacement Secret was deleted: %v", err)
	}
	if stored.UID != replacementUID {
		t.Fatalf("stored Secret UID = %q, want replacement %q", stored.UID, replacementUID)
	}
}

func TestGatewayActionListJWTConsumersDoesNotExposeCredentials(t *testing.T) {
	const namespace = "team-a"
	managedConsumer := managedGatewayJWTConsumer("orders", namespace, "rbd-jwt-orders")
	managedSecret := managedGatewayJWTSecret("rbd-jwt-orders", namespace, map[string][]byte{
		gatewayJWTSecretDataKey:        []byte("orders-client"),
		gatewayJWTSecretDataSecret:     []byte("must-not-leak-managed-secret"),
		gatewayJWTSecretDataPublicKey:  []byte("must-not-leak-managed-public-key"),
		gatewayJWTSecretDataPrivateKey: []byte("must-not-leak-managed-private-key"),
		gatewayJWTSecretDataAlgorithm:  []byte(gatewayJWTAlgorithmHS256),
	})
	externalConsumer := &v2.ApisixConsumer{
		ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: namespace},
		Spec: v2.ApisixConsumerSpec{AuthParameter: v2.ApisixConsumerAuthParameter{
			JwtAuth: &v2.ApisixConsumerJwtAuth{Value: &v2.ApisixConsumerJwtAuthValue{
				Key:        "external-client",
				Secret:     "must-not-leak-external-secret",
				PublicKey:  "must-not-leak-external-public-key",
				PrivateKey: "must-not-leak-external-private-key",
				Algorithm:  gatewayJWTAlgorithmRS256,
			}},
		}},
	}
	externalSecretConsumer := &v2.ApisixConsumer{
		ObjectMeta: metav1.ObjectMeta{Name: "external-ref", Namespace: namespace},
		Spec: v2.ApisixConsumerSpec{AuthParameter: v2.ApisixConsumerAuthParameter{
			JwtAuth: &v2.ApisixConsumerJwtAuth{
				SecretRef: &corev1.LocalObjectReference{Name: "arbitrary-external-secret"},
			},
		}},
	}
	externalSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "arbitrary-external-secret", Namespace: namespace},
		Data: map[string][]byte{
			gatewayJWTSecretDataKey:       []byte("must-not-project-external-key"),
			gatewayJWTSecretDataAlgorithm: []byte(gatewayJWTAlgorithmHS512),
			gatewayJWTSecretDataSecret:    []byte("must-not-leak-external-ref-secret"),
		},
	}
	keyAuthConsumer := &v2.ApisixConsumer{
		ObjectMeta: metav1.ObjectMeta{Name: "key-auth-only", Namespace: namespace},
		Spec: v2.ApisixConsumerSpec{AuthParameter: v2.ApisixConsumerAuthParameter{
			KeyAuth: &v2.ApisixConsumerKeyAuth{Value: &v2.ApisixConsumerKeyAuthValue{Key: "not-jwt"}},
		}},
	}
	routeA := &v2.ApisixRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "route-a", Namespace: namespace},
		Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{
			{Plugins: []v2.ApisixRoutePlugin{{Name: "proxy-rewrite"}}},
			{Plugins: []v2.ApisixRoutePlugin{{
				Name: gatewayConsumerRestrictionPluginName,
				Config: v2.ApisixRoutePluginConfig{
					"whitelist": []interface{}{"team-a_orders", "team-a_external", 42},
				},
			}}},
		}},
	}
	routeZ := &v2.ApisixRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "route-z", Namespace: namespace},
		Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{Plugins: []v2.ApisixRoutePlugin{{
			Name: gatewayConsumerRestrictionPluginName,
			Config: v2.ApisixRoutePluginConfig{
				"whitelist": []string{"team-a_orders-v2", "team-a_orders"},
			},
		}}}}},
	}
	action := gatewayActionForJWTConsumerTest(
		k8sfake.NewSimpleClientset(managedSecret, externalSecret),
		apisixfake.NewSimpleClientset(managedConsumer, externalConsumer, externalSecretConsumer, keyAuthConsumer, routeZ, routeA),
	)

	got, err := action.ListGatewayJWTConsumers(context.Background(), namespace, "")
	if err != nil {
		t.Fatalf("ListGatewayJWTConsumers() error = %v", err)
	}
	if len(got) != 3 || got[0].Name != "external" || got[1].Name != "external-ref" || got[2].Name != "orders" {
		t.Fatalf("listed consumers = %#v, want external, external-ref, then orders", got)
	}
	if !reflect.DeepEqual(got[0].BoundRoutes, []string{"route-a"}) {
		t.Fatalf("external bound routes = %#v", got[0].BoundRoutes)
	}
	if got[0].Source != gatewayJWTExternalSource || got[0].Key != "" || got[0].Algorithm != "" {
		t.Fatalf("external inline credential metadata was projected: %#v", got[0])
	}
	if got[1].Source != gatewayJWTExternalSource || got[1].Key != "" || got[1].Algorithm != "" {
		t.Fatalf("external SecretRef credential metadata was projected: %#v", got[1])
	}
	if !reflect.DeepEqual(got[2].BoundRoutes, []string{"route-a", "route-z"}) {
		t.Fatalf("managed bound routes = %#v", got[2].BoundRoutes)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal listed consumers: %v", err)
	}
	for _, forbidden := range []string{
		"must-not-leak-managed-secret",
		"must-not-leak-managed-public-key",
		"must-not-leak-managed-private-key",
		"must-not-leak-external-secret",
		"must-not-leak-external-public-key",
		"must-not-leak-external-private-key",
		"must-not-leak-external-ref-secret",
		"must-not-project-external-key",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("list response leaked %q: %s", forbidden, encoded)
		}
	}
	var secretLists int
	for _, clientAction := range action.kubeClient.(*k8sfake.Clientset).Actions() {
		if clientAction.GetResource().Resource != "secrets" {
			continue
		}
		if clientAction.GetVerb() == "get" {
			t.Fatalf("list consumers performed an arbitrary Secret GET: %#v", clientAction)
		}
		if clientAction.GetVerb() == "list" {
			secretLists++
			listAction := clientAction.(k8stesting.ListAction)
			selector := listAction.GetListRestrictions().Labels
			selectorText := selector.String()
			if !strings.Contains(selectorText, gatewayJWTManagedByLabel+"="+gatewayJWTManagedValue) ||
				!strings.Contains(selectorText, gatewayJWTAuthTypeLabel+"="+gatewayJWTAuthTypeValue) {
				t.Fatalf("managed Secret selector = %q", selectorText)
			}
		}
	}
	if secretLists != 1 {
		t.Fatalf("Secret list count = %d, want 1", secretLists)
	}

	t.Run("application scope includes shared external owned and bound consumers", func(t *testing.T) {
		shared := managedGatewayJWTConsumer("shared", namespace, "rbd-jwt-shared")
		current := managedGatewayJWTConsumer("current", namespace, "rbd-jwt-current")
		current.Labels["app_id"] = "app-a"
		other := managedGatewayJWTConsumer("other", namespace, "rbd-jwt-other")
		other.Labels["app_id"] = "app-b"
		boundOther := managedGatewayJWTConsumer("bound-other", namespace, "rbd-jwt-bound-other")
		boundOther.Labels["app_id"] = "app-b"
		mixedOther := managedGatewayJWTConsumer("mixed-other", namespace, "rbd-jwt-mixed-other")
		mixedOther.Labels["app_id"] = "app-b"
		mixedSecret := managedGatewayJWTSecret("rbd-jwt-mixed-other", namespace, nil)
		mixedSecret.Labels = nil
		crossOther := managedGatewayJWTConsumer("cross-other", namespace, "rbd-jwt-shared-target")
		crossOther.Labels["app_id"] = "app-b"
		crossSecret := managedGatewayJWTSecret("rbd-jwt-shared-target", namespace, nil)
		appLabelMismatch := managedGatewayJWTConsumer("app-label-mismatch", namespace, "rbd-jwt-app-label-mismatch")
		appLabelMismatch.Labels["app_id"] = "app-b"
		appLabelMismatchSecret := managedGatewayJWTSecret("rbd-jwt-app-label-mismatch", namespace, nil)
		appLabelMismatchSecret.Labels["app_id"] = "app-a"
		currentSecret := managedGatewayJWTSecret("rbd-jwt-current", namespace, nil)
		currentSecret.Labels["app_id"] = "app-a"
		otherSecret := managedGatewayJWTSecret("rbd-jwt-other", namespace, nil)
		otherSecret.Labels["app_id"] = "app-b"
		boundOtherSecret := managedGatewayJWTSecret("rbd-jwt-bound-other", namespace, nil)
		boundOtherSecret.Labels["app_id"] = "app-b"
		external := externalConsumer.DeepCopy()
		external.Name = "external-app"
		boundRoute := &v2.ApisixRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-a-route",
				Namespace: namespace,
				Labels:    map[string]string{"app_id": "app-a"},
			},
			Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{Plugins: []v2.ApisixRoutePlugin{{
				Name: gatewayConsumerRestrictionPluginName,
				Config: v2.ApisixRoutePluginConfig{
					"whitelist": []string{"team-a_bound-other"},
				},
			}}}}},
		}
		otherAppRoute := &v2.ApisixRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-b-route",
				Namespace: namespace,
				Labels:    map[string]string{"app_id": "app-b"},
			},
			Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{Plugins: []v2.ApisixRoutePlugin{{
				Name: gatewayConsumerRestrictionPluginName,
				Config: v2.ApisixRoutePluginConfig{
					"whitelist": []string{"team-a_bound-other"},
				},
			}}}}},
		}
		scopedAction := gatewayActionForJWTConsumerTest(
			k8sfake.NewSimpleClientset(
				managedGatewayJWTSecret("rbd-jwt-shared", namespace, nil),
				currentSecret,
				otherSecret,
				boundOtherSecret,
				mixedSecret,
				crossSecret,
				appLabelMismatchSecret,
			),
			apisixfake.NewSimpleClientset(shared, current, other, boundOther, mixedOther, crossOther, appLabelMismatch, external, boundRoute, otherAppRoute),
		)

		scoped, err := scopedAction.ListGatewayJWTConsumers(context.Background(), namespace, "app-a")
		if err != nil {
			t.Fatalf("list application-scoped consumers: %v", err)
		}
		var names []string
		for _, item := range scoped {
			names = append(names, item.Name)
		}
		wantNames := []string{"app-label-mismatch", "bound-other", "cross-other", "current", "external-app", "mixed-other", "shared"}
		if !reflect.DeepEqual(names, wantNames) {
			t.Fatalf("application-scoped consumer names = %#v, want %#v", names, wantNames)
		}
		for _, item := range scoped {
			wantSource := gatewayJWTManagedValue
			if item.Name == "app-label-mismatch" || item.Name == "cross-other" || item.Name == "external-app" || item.Name == "mixed-other" {
				wantSource = gatewayJWTExternalSource
			}
			if item.Source != wantSource {
				t.Fatalf("consumer %s source = %q, want %q", item.Name, item.Source, wantSource)
			}
			if item.Name == "bound-other" && !reflect.DeepEqual(item.BoundRoutes, []string{"app-a-route"}) {
				t.Fatalf("application-scoped bound routes = %#v, want only app-a-route", item.BoundRoutes)
			}
		}
	})
}

func TestGatewayActionRotateJWTConsumer(t *testing.T) {
	const (
		name       = "orders"
		namespace  = "team-a"
		secretName = "rbd-jwt-orders"
	)
	consumer := managedGatewayJWTConsumer(name, namespace, secretName)
	secret := managedGatewayJWTSecret(secretName, namespace, map[string][]byte{
		gatewayJWTSecretDataKey:       []byte("old-key"),
		gatewayJWTSecretDataSecret:    []byte(strings.Repeat("o", gatewayJWTMinimumSecretBytes)),
		gatewayJWTSecretDataAlgorithm: []byte(gatewayJWTAlgorithmHS256),
	})
	secret.Annotations = map[string]string{"keep": "annotation"}
	kubeClient := k8sfake.NewSimpleClientset(secret)
	apisixClient := apisixfake.NewSimpleClientset(consumer)
	action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)

	got, err := action.RotateGatewayJWTConsumer(context.Background(), namespace, name, &apimodel.GatewayJWTConsumerCredential{
		Key:       "new-key",
		Algorithm: gatewayJWTAlgorithmHS512,
	})
	if err != nil {
		t.Fatalf("RotateGatewayJWTConsumer() error = %v", err)
	}
	if got.GeneratedSecret == "" {
		t.Fatal("generated rotation secret was not returned once")
	}
	if got.Key != "new-key" || got.Algorithm != gatewayJWTAlgorithmHS512 {
		t.Fatalf("rotation response safe metadata = %#v", got)
	}
	updated, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get rotated Secret: %v", err)
	}
	if string(updated.Data[gatewayJWTSecretDataSecret]) != got.GeneratedSecret {
		t.Fatal("rotated Secret does not contain the generated response value")
	}
	if updated.Annotations["keep"] != "annotation" || !isManagedGatewayJWTResource(updated.Labels) {
		t.Fatalf("rotation did not preserve Secret metadata: %#v", updated.ObjectMeta)
	}
	for _, clientAction := range apisixClient.Actions() {
		if clientAction.GetVerb() == "update" || clientAction.GetVerb() == "create" {
			t.Fatalf("rotation unexpectedly changed ApisixConsumer: %#v", clientAction)
		}
	}

	t.Run("external consumer is read only", func(t *testing.T) {
		external := managedGatewayJWTConsumer("external", namespace, "external-secret")
		external.Labels = nil
		externalSecret := managedGatewayJWTSecret("external-secret", namespace, map[string][]byte{
			gatewayJWTSecretDataKey: []byte("external-key"),
		})
		externalAction := gatewayActionForJWTConsumerTest(
			k8sfake.NewSimpleClientset(externalSecret),
			apisixfake.NewSimpleClientset(external),
		)
		_, err := externalAction.RotateGatewayJWTConsumer(context.Background(), namespace, external.Name, &apimodel.GatewayJWTConsumerCredential{
			Key: "new-key",
		})
		if !errors.Is(err, ErrGatewayJWTConsumerNotManaged) {
			t.Fatalf("rotate external consumer error = %v", err)
		}
	})

	t.Run("externally owned secret is read only", func(t *testing.T) {
		managed := managedGatewayJWTConsumer("mixed", namespace, "mixed-secret")
		externalSecret := managedGatewayJWTSecret("mixed-secret", namespace, nil)
		externalSecret.Labels = nil
		mixedAction := gatewayActionForJWTConsumerTest(
			k8sfake.NewSimpleClientset(externalSecret),
			apisixfake.NewSimpleClientset(managed),
		)
		_, err := mixedAction.RotateGatewayJWTConsumer(context.Background(), namespace, managed.Name, &apimodel.GatewayJWTConsumerCredential{
			Key: "new-key",
		})
		if !errors.Is(err, ErrGatewayJWTConsumerNotManaged) {
			t.Fatalf("rotate externally owned Secret error = %v", err)
		}
	})

	t.Run("another consumers managed secret is read only", func(t *testing.T) {
		crossConsumer := managedGatewayJWTConsumer("cross", namespace, "rbd-jwt-victim")
		victimSecret := managedGatewayJWTSecret("rbd-jwt-victim", namespace, map[string][]byte{
			gatewayJWTSecretDataKey:    []byte("victim-key"),
			gatewayJWTSecretDataSecret: []byte(strings.Repeat("v", gatewayJWTMinimumSecretBytes)),
		})
		crossAction := gatewayActionForJWTConsumerTest(
			k8sfake.NewSimpleClientset(victimSecret),
			apisixfake.NewSimpleClientset(crossConsumer),
		)
		_, err := crossAction.RotateGatewayJWTConsumer(context.Background(), namespace, crossConsumer.Name, &apimodel.GatewayJWTConsumerCredential{
			Key:       "attacker-key",
			Secret:    strings.Repeat("a", gatewayJWTMinimumSecretBytes),
			Algorithm: gatewayJWTAlgorithmHS256,
		})
		if !errors.Is(err, ErrGatewayJWTConsumerNotManaged) {
			t.Fatalf("rotate cross-consumer Secret reference error = %v", err)
		}
		stored, getErr := crossAction.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), victimSecret.Name, metav1.GetOptions{})
		if getErr != nil {
			t.Fatalf("get victim Secret: %v", getErr)
		}
		if string(stored.Data[gatewayJWTSecretDataKey]) != "victim-key" {
			t.Fatal("cross-consumer rotation changed the victim Secret")
		}
	})

	t.Run("mismatched application ownership is read only", func(t *testing.T) {
		appConsumer := managedGatewayJWTConsumer("app-owned", namespace, "rbd-jwt-app-owned")
		appConsumer.Labels["app_id"] = "app-a"
		appSecret := managedGatewayJWTSecret("rbd-jwt-app-owned", namespace, nil)
		appSecret.Labels["app_id"] = "app-b"
		appAction := gatewayActionForJWTConsumerTest(
			k8sfake.NewSimpleClientset(appSecret),
			apisixfake.NewSimpleClientset(appConsumer),
		)
		_, err := appAction.RotateGatewayJWTConsumer(context.Background(), namespace, appConsumer.Name, &apimodel.GatewayJWTConsumerCredential{
			Key: "new-key",
		})
		if !errors.Is(err, ErrGatewayJWTConsumerNotManaged) {
			t.Fatalf("rotate app-mismatched Secret error = %v", err)
		}
	})
}

func TestGatewayActionDeleteJWTConsumerChecksBindings(t *testing.T) {
	const (
		name       = "orders"
		namespace  = "team-a"
		secretName = "rbd-jwt-orders"
	)
	consumer := managedGatewayJWTConsumer(name, namespace, secretName)
	consumer.UID = types.UID("consumer-uid")
	secret := managedGatewayJWTSecret(secretName, namespace, map[string][]byte{
		gatewayJWTSecretDataKey: []byte("orders-client"),
	})
	secret.UID = types.UID("secret-uid")
	boundRoute := &v2.ApisixRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "bound", Namespace: namespace},
		Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{Plugins: []v2.ApisixRoutePlugin{{
			Name: gatewayConsumerRestrictionPluginName,
			Config: v2.ApisixRoutePluginConfig{
				"whitelist": []interface{}{"team-a_orders"},
			},
		}}}}},
	}
	almostRoute := &v2.ApisixRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "almost", Namespace: namespace},
		Spec: v2.ApisixRouteSpec{HTTP: []v2.ApisixRouteHTTP{{Plugins: []v2.ApisixRoutePlugin{{
			Name: gatewayConsumerRestrictionPluginName,
			Config: v2.ApisixRoutePluginConfig{
				"whitelist": []string{"team-a_orders-v2"},
			},
		}}}}},
	}
	kubeClient := k8sfake.NewSimpleClientset(secret)
	apisixClient := apisixfake.NewSimpleClientset(consumer, boundRoute, almostRoute)
	action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)

	err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, name)
	if !errors.Is(err, ErrGatewayJWTConsumerInUse) {
		t.Fatalf("delete bound consumer error = %v", err)
	}
	if _, err := apisixClient.ApisixV2().ApisixConsumers(namespace).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
		t.Fatalf("bound consumer was changed: %v", err)
	}
	if _, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{}); err != nil {
		t.Fatalf("bound Secret was changed: %v", err)
	}

	if err := apisixClient.ApisixV2().ApisixRoutes(namespace).Delete(context.Background(), boundRoute.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("remove exact binding fixture: %v", err)
	}
	var deletionOrder []string
	apisixClient.PrependReactor("delete", "apisixconsumers", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if uid := gatewayJWTDeleteUID(t, action); uid != consumer.UID {
			t.Fatalf("Consumer delete UID = %q, want %q", uid, consumer.UID)
		}
		deletionOrder = append(deletionOrder, "ApisixConsumer")
		return false, nil, nil
	})
	kubeClient.PrependReactor("delete", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if uid := gatewayJWTDeleteUID(t, action); uid != secret.UID {
			t.Fatalf("Secret delete UID = %q, want %q", uid, secret.UID)
		}
		deletionOrder = append(deletionOrder, "Secret")
		return false, nil, nil
	})
	if err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, name); err != nil {
		t.Fatalf("delete unbound consumer: %v", err)
	}
	if !reflect.DeepEqual(deletionOrder, []string{"ApisixConsumer", "Secret"}) {
		t.Fatalf("deletion order = %#v, want ApisixConsumer then Secret", deletionOrder)
	}
}

func TestGatewayActionDeleteJWTConsumerRejectsExternalResource(t *testing.T) {
	const namespace = "team-a"
	external := managedGatewayJWTConsumer("external", namespace, "external-secret")
	external.Labels = nil
	externalSecret := managedGatewayJWTSecret("external-secret", namespace, nil)
	action := gatewayActionForJWTConsumerTest(
		k8sfake.NewSimpleClientset(externalSecret),
		apisixfake.NewSimpleClientset(external),
	)

	err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, external.Name)
	if !errors.Is(err, ErrGatewayJWTConsumerNotManaged) {
		t.Fatalf("delete external consumer error = %v", err)
	}
}

func TestGatewayActionDeleteJWTConsumerRejectsCrossConsumerSecretReference(t *testing.T) {
	const namespace = "team-a"
	crossConsumer := managedGatewayJWTConsumer("cross", namespace, "rbd-jwt-victim")
	victimSecret := managedGatewayJWTSecret("rbd-jwt-victim", namespace, nil)
	kubeClient := k8sfake.NewSimpleClientset(victimSecret)
	apisixClient := apisixfake.NewSimpleClientset(crossConsumer)
	action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)

	err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, crossConsumer.Name)
	if !errors.Is(err, ErrGatewayJWTConsumerNotManaged) {
		t.Fatalf("delete cross-consumer Secret reference error = %v", err)
	}
	if _, err := apisixClient.ApisixV2().ApisixConsumers(namespace).Get(context.Background(), crossConsumer.Name, metav1.GetOptions{}); err != nil {
		t.Fatalf("cross-referencing Consumer was deleted: %v", err)
	}
	if _, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), victimSecret.Name, metav1.GetOptions{}); err != nil {
		t.Fatalf("victim Secret was deleted: %v", err)
	}
}

func TestGatewayActionDeleteJWTConsumerRetriesOrphanSecret(t *testing.T) {
	const (
		name       = "orders"
		namespace  = "team-a"
		secretName = "rbd-jwt-orders"
	)
	consumer := managedGatewayJWTConsumer(name, namespace, secretName)
	consumer.UID = types.UID("consumer-uid")
	secret := managedGatewayJWTSecret(secretName, namespace, nil)
	secret.UID = types.UID("secret-uid")
	kubeClient := k8sfake.NewSimpleClientset(secret)
	apisixClient := apisixfake.NewSimpleClientset(consumer)
	deleteAttempts := 0
	kubeClient.PrependReactor("delete", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if uid := gatewayJWTDeleteUID(t, action); uid != secret.UID {
			t.Fatalf("Secret delete UID = %q, want %q", uid, secret.UID)
		}
		deleteAttempts++
		if deleteAttempts == 1 {
			return true, nil, errors.New("transient Secret delete failure")
		}
		return false, nil, nil
	})
	action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)

	if err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, name); err == nil {
		t.Fatal("first delete unexpectedly succeeded")
	}
	if _, err := apisixClient.ApisixV2().ApisixConsumers(namespace).Get(context.Background(), name, metav1.GetOptions{}); !k8serrors.IsNotFound(err) {
		t.Fatalf("Consumer remained after successful first-stage deletion: %v", err)
	}
	if _, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{}); err != nil {
		t.Fatalf("orphan Secret missing before retry: %v", err)
	}
	if err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, name); err != nil {
		t.Fatalf("retry orphan Secret deletion: %v", err)
	}
	if _, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{}); !k8serrors.IsNotFound(err) {
		t.Fatalf("orphan Secret remained after retry: %v", err)
	}
}

func TestGatewayActionDeleteJWTConsumerPreservesReferencedOrphanSecret(t *testing.T) {
	const (
		missingName = "orders"
		namespace   = "team-a"
		secretName  = "rbd-jwt-orders"
	)
	secret := managedGatewayJWTSecret(secretName, namespace, nil)
	otherConsumer := &v2.ApisixConsumer{
		ObjectMeta: metav1.ObjectMeta{Name: "external-client", Namespace: namespace},
		Spec: v2.ApisixConsumerSpec{AuthParameter: v2.ApisixConsumerAuthParameter{
			JwtAuth: &v2.ApisixConsumerJwtAuth{
				SecretRef: &corev1.LocalObjectReference{Name: secretName},
			},
		}},
	}
	kubeClient := k8sfake.NewSimpleClientset(secret)
	action := gatewayActionForJWTConsumerTest(
		kubeClient,
		apisixfake.NewSimpleClientset(otherConsumer),
	)

	err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, missingName)
	if !errors.Is(err, ErrGatewayJWTConsumerInUse) {
		t.Fatalf("delete referenced orphan Secret error = %v", err)
	}
	if _, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{}); err != nil {
		t.Fatalf("referenced orphan Secret was deleted: %v", err)
	}
}

func TestGatewayActionDeleteJWTConsumerPreservesSharedSecret(t *testing.T) {
	const (
		name       = "orders"
		namespace  = "team-a"
		secretName = "rbd-jwt-orders"
	)
	consumer := managedGatewayJWTConsumer(name, namespace, secretName)
	consumer.UID = types.UID("orders-consumer-uid")
	secret := managedGatewayJWTSecret(secretName, namespace, nil)
	secret.UID = types.UID("orders-secret-uid")
	otherConsumer := &v2.ApisixConsumer{
		ObjectMeta: metav1.ObjectMeta{Name: "external-client", Namespace: namespace},
		Spec: v2.ApisixConsumerSpec{AuthParameter: v2.ApisixConsumerAuthParameter{
			JwtAuth: &v2.ApisixConsumerJwtAuth{
				SecretRef: &corev1.LocalObjectReference{Name: secretName},
			},
		}},
	}
	kubeClient := k8sfake.NewSimpleClientset(secret)
	apisixClient := apisixfake.NewSimpleClientset(consumer, otherConsumer)
	action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)

	err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, name)
	if !errors.Is(err, ErrGatewayJWTConsumerInUse) {
		t.Fatalf("delete shared Secret error = %v", err)
	}
	if _, err := apisixClient.ApisixV2().ApisixConsumers(namespace).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
		t.Fatalf("owner Consumer was deleted before shared reference rejection: %v", err)
	}
	if _, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{}); err != nil {
		t.Fatalf("shared Secret was deleted: %v", err)
	}
}

func TestGatewayActionDeleteJWTConsumerIsIdempotent(t *testing.T) {
	const (
		name       = "orders"
		namespace  = "team-a"
		secretName = "rbd-jwt-orders"
	)
	t.Run("both resources absent", func(t *testing.T) {
		action := gatewayActionForJWTConsumerTest(k8sfake.NewSimpleClientset(), apisixfake.NewSimpleClientset())
		if err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, name); err != nil {
			t.Fatalf("delete absent Consumer and Secret: %v", err)
		}
	})

	t.Run("Consumer delete NotFound continues Secret cleanup", func(t *testing.T) {
		consumer := managedGatewayJWTConsumer(name, namespace, secretName)
		consumer.UID = types.UID("consumer-uid")
		secret := managedGatewayJWTSecret(secretName, namespace, nil)
		secret.UID = types.UID("secret-uid")
		kubeClient := k8sfake.NewSimpleClientset(secret)
		apisixClient := apisixfake.NewSimpleClientset(consumer)
		apisixClient.PrependReactor("delete", "apisixconsumers", func(action k8stesting.Action) (bool, runtime.Object, error) {
			if uid := gatewayJWTDeleteUID(t, action); uid != consumer.UID {
				t.Fatalf("Consumer delete UID = %q, want %q", uid, consumer.UID)
			}
			if err := apisixClient.Tracker().Delete(v2.SchemeGroupVersion.WithResource("apisixconsumers"), namespace, name); err != nil {
				t.Fatalf("simulate concurrent Consumer deletion: %v", err)
			}
			return true, nil, k8serrors.NewNotFound(v2.Resource("apisixconsumers"), name)
		})
		kubeClient.PrependReactor("delete", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
			if uid := gatewayJWTDeleteUID(t, action); uid != secret.UID {
				t.Fatalf("Secret delete UID = %q, want %q", uid, secret.UID)
			}
			return false, nil, nil
		})
		action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)

		if err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, name); err != nil {
			t.Fatalf("delete after concurrent Consumer deletion: %v", err)
		}
		if _, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{}); !k8serrors.IsNotFound(err) {
			t.Fatalf("Secret remained after Consumer delete NotFound: %v", err)
		}
	})

	t.Run("Secret delete NotFound is success", func(t *testing.T) {
		consumer := managedGatewayJWTConsumer(name, namespace, secretName)
		consumer.UID = types.UID("consumer-uid")
		secret := managedGatewayJWTSecret(secretName, namespace, nil)
		secret.UID = types.UID("secret-uid")
		kubeClient := k8sfake.NewSimpleClientset(secret)
		apisixClient := apisixfake.NewSimpleClientset(consumer)
		kubeClient.PrependReactor("delete", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
			if uid := gatewayJWTDeleteUID(t, action); uid != secret.UID {
				t.Fatalf("Secret delete UID = %q, want %q", uid, secret.UID)
			}
			if err := kubeClient.Tracker().Delete(corev1.SchemeGroupVersion.WithResource("secrets"), namespace, secretName); err != nil {
				t.Fatalf("simulate concurrent Secret deletion: %v", err)
			}
			return true, nil, k8serrors.NewNotFound(corev1.Resource("secrets"), secretName)
		})
		action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)

		if err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, name); err != nil {
			t.Fatalf("delete after concurrent Secret deletion: %v", err)
		}
	})
}

func TestGatewayActionDeleteJWTConsumerPreservesUIDReplacements(t *testing.T) {
	const (
		name       = "orders"
		namespace  = "team-a"
		secretName = "rbd-jwt-orders"
	)
	t.Run("Consumer replacement", func(t *testing.T) {
		consumer := managedGatewayJWTConsumer(name, namespace, secretName)
		consumer.UID = types.UID("observed-consumer-uid")
		secret := managedGatewayJWTSecret(secretName, namespace, nil)
		secret.UID = types.UID("secret-uid")
		kubeClient := k8sfake.NewSimpleClientset(secret)
		apisixClient := apisixfake.NewSimpleClientset(consumer)
		replacementUID := types.UID("replacement-consumer-uid")
		apisixClient.PrependReactor("delete", "apisixconsumers", func(action k8stesting.Action) (bool, runtime.Object, error) {
			if uid := gatewayJWTDeleteUID(t, action); uid != consumer.UID {
				t.Fatalf("Consumer delete UID = %q, want %q", uid, consumer.UID)
			}
			replacement := consumer.DeepCopy()
			replacement.UID = replacementUID
			if err := apisixClient.Tracker().Update(v2.SchemeGroupVersion.WithResource("apisixconsumers"), replacement, namespace); err != nil {
				t.Fatalf("replace Consumer during delete: %v", err)
			}
			return true, nil, k8serrors.NewConflict(v2.Resource("apisixconsumers"), name, errors.New("UID precondition failed"))
		})
		action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)

		err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, name)
		if !k8serrors.IsConflict(err) {
			t.Fatalf("Consumer replacement error = %v, want Conflict", err)
		}
		stored, err := apisixClient.ApisixV2().ApisixConsumers(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil || stored.UID != replacementUID {
			t.Fatalf("replacement Consumer was not preserved: consumer=%#v err=%v", stored, err)
		}
	})

	t.Run("Secret replacement", func(t *testing.T) {
		consumer := managedGatewayJWTConsumer(name, namespace, secretName)
		consumer.UID = types.UID("consumer-uid")
		secret := managedGatewayJWTSecret(secretName, namespace, nil)
		secret.UID = types.UID("observed-secret-uid")
		kubeClient := k8sfake.NewSimpleClientset(secret)
		apisixClient := apisixfake.NewSimpleClientset(consumer)
		replacementUID := types.UID("replacement-secret-uid")
		kubeClient.PrependReactor("delete", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
			if uid := gatewayJWTDeleteUID(t, action); uid != secret.UID {
				t.Fatalf("Secret delete UID = %q, want %q", uid, secret.UID)
			}
			replacement := secret.DeepCopy()
			replacement.UID = replacementUID
			if err := kubeClient.Tracker().Update(corev1.SchemeGroupVersion.WithResource("secrets"), replacement, namespace); err != nil {
				t.Fatalf("replace Secret during delete: %v", err)
			}
			return true, nil, k8serrors.NewConflict(corev1.Resource("secrets"), secretName, errors.New("UID precondition failed"))
		})
		action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)

		err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, name)
		if !k8serrors.IsConflict(err) {
			t.Fatalf("Secret replacement error = %v, want Conflict", err)
		}
		stored, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
		if err != nil || stored.UID != replacementUID {
			t.Fatalf("replacement Secret was not preserved: secret=%#v err=%v", stored, err)
		}
	})
}

func TestGatewayActionDeleteJWTConsumerPreservesSameNameNewUIDReference(t *testing.T) {
	const (
		name       = "orders"
		namespace  = "team-a"
		secretName = "rbd-jwt-orders"
	)
	original := managedGatewayJWTConsumer(name, namespace, secretName)
	original.UID = types.UID("original-consumer-uid")
	replacement := managedGatewayJWTConsumer(name, namespace, secretName)
	replacement.UID = types.UID("replacement-consumer-uid")
	secret := managedGatewayJWTSecret(secretName, namespace, nil)
	secret.UID = types.UID("secret-uid")
	kubeClient := k8sfake.NewSimpleClientset(secret)
	apisixClient := apisixfake.NewSimpleClientset(original)
	consumerListCalls := 0
	apisixClient.PrependReactor("list", "apisixconsumers", func(k8stesting.Action) (bool, runtime.Object, error) {
		consumerListCalls++
		if consumerListCalls == 2 {
			if err := apisixClient.Tracker().Add(replacement); err != nil {
				t.Fatalf("insert same-name replacement Consumer: %v", err)
			}
		}
		return false, nil, nil
	})
	action := gatewayActionForJWTConsumerTest(kubeClient, apisixClient)

	err := action.DeleteGatewayJWTConsumer(context.Background(), namespace, name)
	if !errors.Is(err, ErrGatewayJWTConsumerInUse) {
		t.Fatalf("same-name replacement reference error = %v", err)
	}
	storedConsumer, err := apisixClient.ApisixV2().ApisixConsumers(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil || storedConsumer.UID != replacement.UID {
		t.Fatalf("same-name replacement Consumer was not preserved: consumer=%#v err=%v", storedConsumer, err)
	}
	if _, err := kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{}); err != nil {
		t.Fatalf("Secret referenced by same-name replacement was deleted: %v", err)
	}
}

func TestGatewayActionConfigureManagedJWTConsumer(t *testing.T) {
	const namespace = "team-a"
	managed := managedGatewayJWTConsumer("orders", namespace, "rbd-jwt-orders")
	external := &v2.ApisixConsumer{
		ObjectMeta: metav1.ObjectMeta{Name: "partner", Namespace: namespace},
		Spec: v2.ApisixConsumerSpec{AuthParameter: v2.ApisixConsumerAuthParameter{
			JwtAuth: &v2.ApisixConsumerJwtAuth{Value: &v2.ApisixConsumerJwtAuthValue{Key: "partner-key"}},
		}},
	}
	nonJWT := &v2.ApisixConsumer{
		ObjectMeta: metav1.ObjectMeta{Name: "basic", Namespace: namespace},
		Spec: v2.ApisixConsumerSpec{AuthParameter: v2.ApisixConsumerAuthParameter{
			BasicAuth: &v2.ApisixConsumerBasicAuth{Value: &v2.ApisixConsumerBasicAuthValue{Username: "basic"}},
		}},
	}
	action := gatewayActionForJWTConsumerTest(
		k8sfake.NewSimpleClientset(),
		apisixfake.NewSimpleClientset(managed, external, nonJWT),
	)
	existing := []v2.ApisixRoutePlugin{
		{Name: "proxy-rewrite", Enable: true, Config: v2.ApisixRoutePluginConfig{"uri": "/v1"}},
		{Name: gatewayJWTAuthPluginName, Enable: true, Config: v2.ApisixRoutePluginConfig{"header": "legacy"}},
	}

	got, err := action.ConfigureManagedJWTAuth(context.Background(), namespace, "", &apimodel.ManagedJWTAuthentication{
		Enabled:       true,
		ConsumerNames: []string{"orders", "partner", "orders"},
		Config:        v2.ApisixRoutePluginConfig{"header": "authorization"},
	}, existing)
	if err != nil {
		t.Fatalf("ConfigureManagedJWTAuth() error = %v", err)
	}
	want := BuildManagedJWTPlugins(namespace, []string{"orders", "partner", "orders"}, v2.ApisixRoutePluginConfig{"header": "authorization"}, existing)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfigureManagedJWTAuth() = %#v, want %#v", got, want)
	}

	t.Run("nil authentication preserves legacy plugins without aliases", func(t *testing.T) {
		preserved, err := action.ConfigureManagedJWTAuth(context.Background(), namespace, "", nil, existing)
		if err != nil {
			t.Fatalf("preserve legacy plugins: %v", err)
		}
		if !reflect.DeepEqual(preserved, existing) {
			t.Fatalf("preserved plugins = %#v, want %#v", preserved, existing)
		}
		preserved[0].Config["uri"] = "/changed"
		if existing[0].Config["uri"] != "/v1" {
			t.Fatal("preserved plugin config aliases the input")
		}
	})

	t.Run("disabled removes only managed plugins", func(t *testing.T) {
		disabled, err := action.ConfigureManagedJWTAuth(context.Background(), namespace, "", &apimodel.ManagedJWTAuthentication{}, existing)
		if err != nil {
			t.Fatalf("disable managed JWT: %v", err)
		}
		if len(disabled) != 1 || disabled[0].Name != "proxy-rewrite" {
			t.Fatalf("disabled plugins = %#v", disabled)
		}
	})

	t.Run("empty selection is rejected", func(t *testing.T) {
		_, err := action.ConfigureManagedJWTAuth(context.Background(), namespace, "", &apimodel.ManagedJWTAuthentication{Enabled: true}, existing)
		if !errors.Is(err, ErrGatewayJWTConsumerRequired) {
			t.Fatalf("empty selection error = %v", err)
		}
	})

	t.Run("missing consumer is rejected", func(t *testing.T) {
		_, err := action.ConfigureManagedJWTAuth(context.Background(), namespace, "", &apimodel.ManagedJWTAuthentication{
			Enabled:       true,
			ConsumerNames: []string{"missing"},
		}, existing)
		if !k8serrors.IsNotFound(err) {
			t.Fatalf("missing consumer error = %v", err)
		}
	})

	t.Run("non JWT consumer is rejected", func(t *testing.T) {
		_, err := action.ConfigureManagedJWTAuth(context.Background(), namespace, "", &apimodel.ManagedJWTAuthentication{
			Enabled:       true,
			ConsumerNames: []string{"basic"},
		}, existing)
		if !errors.Is(err, ErrGatewayJWTConsumerNotJWT) {
			t.Fatalf("non-JWT consumer error = %v", err)
		}
	})

	t.Run("application scope allows shared matching and external consumers", func(t *testing.T) {
		shared := managedGatewayJWTConsumer("shared", namespace, "rbd-jwt-shared")
		matching := managedGatewayJWTConsumer("matching", namespace, "rbd-jwt-matching")
		matching.Labels["app_id"] = "app-a"
		matchingSecret := managedGatewayJWTSecret("rbd-jwt-matching", namespace, nil)
		matchingSecret.Labels["app_id"] = "app-a"
		other := managedGatewayJWTConsumer("other", namespace, "rbd-jwt-other")
		other.Labels["app_id"] = "app-b"
		otherSecret := managedGatewayJWTSecret("rbd-jwt-other", namespace, nil)
		otherSecret.Labels["app_id"] = "app-b"
		mixedOther := managedGatewayJWTConsumer("mixed-other-app", namespace, "rbd-jwt-mixed-other-app")
		mixedOther.Labels["app_id"] = "app-b"
		appAction := gatewayActionForJWTConsumerTest(
			k8sfake.NewSimpleClientset(
				managedGatewayJWTSecret("rbd-jwt-shared", namespace, nil),
				matchingSecret,
				otherSecret,
			),
			apisixfake.NewSimpleClientset(shared, matching, other, mixedOther, external),
		)

		_, err := appAction.ConfigureManagedJWTAuth(context.Background(), namespace, "app-a", &apimodel.ManagedJWTAuthentication{
			Enabled:       true,
			ConsumerNames: []string{"shared", "matching", "partner"},
		}, existing)
		if err != nil {
			t.Fatalf("configure allowed app consumers: %v", err)
		}

		_, err = appAction.ConfigureManagedJWTAuth(context.Background(), namespace, "app-a", &apimodel.ManagedJWTAuthentication{
			Enabled:       true,
			ConsumerNames: []string{"other"},
		}, existing)
		if !errors.Is(err, ErrGatewayJWTConsumerAppMismatch) {
			t.Fatalf("cross-app managed consumer error = %v", err)
		}

		_, err = appAction.ConfigureManagedJWTAuth(context.Background(), namespace, "app-a", &apimodel.ManagedJWTAuthentication{
			Enabled:       true,
			ConsumerNames: []string{"mixed-other-app"},
		}, existing)
		if !errors.Is(err, ErrGatewayJWTConsumerAppMismatch) {
			t.Fatalf("cross-app mixed-ownership consumer error = %v", err)
		}
	})
}

func TestGatewayActionConfigureManagedJWTConsumerRequiresUsableJWTSource(t *testing.T) {
	const namespace = "team-a"
	tests := []struct {
		name      string
		jwtAuth   *v2.ApisixConsumerJwtAuth
		wantError bool
	}{
		{name: "missing auth", wantError: true},
		{name: "empty inline key", jwtAuth: &v2.ApisixConsumerJwtAuth{Value: &v2.ApisixConsumerJwtAuthValue{}}, wantError: true},
		{name: "empty Secret reference", jwtAuth: &v2.ApisixConsumerJwtAuth{SecretRef: &corev1.LocalObjectReference{}}, wantError: true},
		{name: "blank Secret reference", jwtAuth: &v2.ApisixConsumerJwtAuth{SecretRef: &corev1.LocalObjectReference{Name: "  "}}, wantError: true},
		{name: "inline key", jwtAuth: &v2.ApisixConsumerJwtAuth{Value: &v2.ApisixConsumerJwtAuthValue{Key: "client"}}},
		{name: "Secret reference", jwtAuth: &v2.ApisixConsumerJwtAuth{SecretRef: &corev1.LocalObjectReference{Name: "consumer-secret"}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			consumer := &v2.ApisixConsumer{
				ObjectMeta: metav1.ObjectMeta{Name: "consumer", Namespace: namespace},
				Spec: v2.ApisixConsumerSpec{AuthParameter: v2.ApisixConsumerAuthParameter{
					JwtAuth: test.jwtAuth,
				}},
			}
			action := gatewayActionForJWTConsumerTest(k8sfake.NewSimpleClientset(), apisixfake.NewSimpleClientset(consumer))
			_, err := action.ConfigureManagedJWTAuth(context.Background(), namespace, "", &apimodel.ManagedJWTAuthentication{
				Enabled:       true,
				ConsumerNames: []string{"consumer"},
			}, nil)
			if test.wantError && !errors.Is(err, ErrGatewayJWTConsumerNotJWT) {
				t.Fatalf("ConfigureManagedJWTAuth() error = %v, want ErrGatewayJWTConsumerNotJWT", err)
			}
			if !test.wantError && err != nil {
				t.Fatalf("ConfigureManagedJWTAuth() error = %v", err)
			}
		})
	}
}

func TestGatewayActionConfigureManagedJWTAuthValidatesPluginConfig(t *testing.T) {
	const namespace = "team-a"
	consumer := &v2.ApisixConsumer{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: namespace},
		Spec: v2.ApisixConsumerSpec{AuthParameter: v2.ApisixConsumerAuthParameter{
			JwtAuth: &v2.ApisixConsumerJwtAuth{Value: &v2.ApisixConsumerJwtAuthValue{Key: "orders-client"}},
		}},
	}
	action := gatewayActionForJWTConsumerTest(k8sfake.NewSimpleClientset(), apisixfake.NewSimpleClientset(consumer))
	tests := []struct {
		name      string
		config    v2.ApisixRoutePluginConfig
		wantError bool
	}{
		{name: "empty config"},
		{name: "supported fields", config: v2.ApisixRoutePluginConfig{
			"header":           "authorization",
			"query":            "jwt",
			"cookie":           "jwt",
			"claims_to_verify": []interface{}{"exp", "nbf"},
		}},
		{name: "typed string claims", config: v2.ApisixRoutePluginConfig{"claims_to_verify": []string{"exp"}}},
		{name: "anonymous consumer", config: v2.ApisixRoutePluginConfig{"anonymous_consumer": "guest"}, wantError: true},
		{name: "unknown field", config: v2.ApisixRoutePluginConfig{"key": "value"}, wantError: true},
		{name: "wrong header type", config: v2.ApisixRoutePluginConfig{"header": true}, wantError: true},
		{name: "blank header", config: v2.ApisixRoutePluginConfig{"header": "  "}, wantError: true},
		{name: "wrong claims type", config: v2.ApisixRoutePluginConfig{"claims_to_verify": "exp"}, wantError: true},
		{name: "non-string claim", config: v2.ApisixRoutePluginConfig{"claims_to_verify": []interface{}{"exp", 1}}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := action.ConfigureManagedJWTAuth(context.Background(), namespace, "", &apimodel.ManagedJWTAuthentication{
				Enabled:       true,
				ConsumerNames: []string{"orders"},
				Config:        test.config,
			}, nil)
			if test.wantError && !errors.Is(err, ErrGatewayJWTAuthInvalidConfig) {
				t.Fatalf("ConfigureManagedJWTAuth() error = %v, want ErrGatewayJWTAuthInvalidConfig", err)
			}
			if !test.wantError && err != nil {
				t.Fatalf("ConfigureManagedJWTAuth() error = %v", err)
			}
		})
	}
}
