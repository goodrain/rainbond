package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	apimodel "github.com/goodrain/rainbond/api/model"
	apiutil "github.com/goodrain/rainbond/api/util"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	gatewayJWTAlgorithmHS256     = "HS256"
	gatewayJWTAlgorithmHS512     = "HS512"
	gatewayJWTAlgorithmRS256     = "RS256"
	gatewayJWTDefaultExpiration  = 86400
	gatewayJWTMinimumSecretBytes = 32

	gatewayJWTAuthPluginName             = "jwt-auth"
	gatewayConsumerRestrictionPluginName = "consumer-restriction"

	gatewayJWTManagedByLabel        = "gateway.rainbond.io/managed-by"
	gatewayJWTAuthTypeLabel         = "gateway.rainbond.io/auth-type"
	gatewayJWTManagedRouteLabel     = "gateway.rainbond.io/managed-jwt"
	gatewayJWTManagedValue          = "rainbond"
	gatewayJWTAuthTypeValue         = "jwt"
	gatewayJWTManagedRouteValue     = "true"
	gatewayJWTExternalSource        = "external"
	gatewayJWTConsumerStatusPending = "pending"

	gatewayJWTSecretDataKey                 = "key"
	gatewayJWTSecretDataSecret              = "secret"
	gatewayJWTSecretDataPublicKey           = "public_key"
	gatewayJWTSecretDataPrivateKey          = "private_key"
	gatewayJWTSecretDataAlgorithm           = "algorithm"
	gatewayJWTSecretDataExp                 = "exp"
	gatewayJWTSecretDataBase64Secret        = "base64_secret"
	gatewayJWTSecretDataLifetimeGracePeriod = "lifetime_grace_period"

	gatewayJWTGeneratedSecretBytes = 32
	gatewayJWTSecretNamePrefix     = "rbd-jwt-"
	gatewayJWTIngressClassName     = "apisix"
	gatewayJWTApplicationLabel     = "app_id"
)

var (
	// ErrGatewayJWTConsumerInvalidName indicates that a Consumer cannot be
	// represented safely as Kubernetes resources.
	ErrGatewayJWTConsumerInvalidName = errors.New("gateway JWT consumer name is invalid")
	// ErrGatewayJWTConsumerInvalidCredential identifies credential input that
	// cannot be persisted as a supported APISIX JWT credential.
	ErrGatewayJWTConsumerInvalidCredential = errors.New("gateway JWT consumer credential is invalid")
	// ErrGatewayJWTConsumerNotManaged protects externally owned credentials
	// from mutation through Rainbond's managed Consumer endpoints.
	ErrGatewayJWTConsumerNotManaged = errors.New("gateway JWT consumer is not managed by Rainbond")
	// ErrGatewayJWTConsumerInUse protects a Consumer referenced by a route.
	ErrGatewayJWTConsumerInUse = errors.New("gateway JWT consumer is bound to one or more routes")
	// ErrGatewayJWTConsumerRequired indicates that managed JWT auth has no
	// Consumer selection.
	ErrGatewayJWTConsumerRequired = errors.New("at least one gateway JWT consumer is required")
	// ErrGatewayJWTConsumerNotJWT indicates a selected Consumer that cannot
	// satisfy jwt-auth.
	ErrGatewayJWTConsumerNotJWT = errors.New("gateway consumer does not define JWT authentication")
	// ErrGatewayJWTConsumerAppMismatch prevents an application from binding a
	// managed Consumer owned by another application.
	ErrGatewayJWTConsumerAppMismatch = errors.New("gateway JWT consumer belongs to another application")
	// ErrGatewayJWTAuthInvalidConfig identifies unsafe or unsupported route
	// plugin fields in managed JWT authentication.
	ErrGatewayJWTAuthInvalidConfig = errors.New("managed gateway JWT authentication config is invalid")

	errGatewayJWTConsumerCredentialRequired = fmt.Errorf("%w: credential is required", ErrGatewayJWTConsumerInvalidCredential)
	errGatewayJWTConsumerKeyRequired        = fmt.Errorf("%w: key is required", ErrGatewayJWTConsumerInvalidCredential)
)

// ListGatewayJWTConsumers returns safe metadata for JWT-capable Consumers.
// Secret and in-place credential material are never copied into the response.
func (g *GatewayAction) ListGatewayJWTConsumers(ctx context.Context, namespace, appID string) ([]*apimodel.GatewayJWTConsumer, error) {
	consumers, err := g.apisixClient.ApisixV2().ApisixConsumers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list APISIX consumers in namespace %q: %w", namespace, err)
	}
	routes, err := g.apisixClient.ApisixV2().ApisixRoutes(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list APISIX routes in namespace %q: %w", namespace, err)
	}
	managedSecrets, err := g.listManagedGatewayJWTSecrets(ctx, namespace)
	if err != nil {
		return nil, err
	}

	result := make([]*apimodel.GatewayJWTConsumer, 0, len(consumers.Items))
	for i := range consumers.Items {
		consumer := &consumers.Items[i]
		jwtAuth := consumer.Spec.AuthParameter.JwtAuth
		if jwtAuth == nil {
			continue
		}

		var secret *corev1.Secret
		expectedSecretName := gatewayJWTConsumerSecretName(consumer.Name)
		if jwtAuth.SecretRef != nil && jwtAuth.SecretRef.Name == expectedSecretName {
			secret = managedSecrets[expectedSecretName]
		}
		boundRoutes, boundToApp := gatewayJWTConsumerBindings(routes.Items, gatewayJWTConsumerUsername(namespace, consumer.Name), appID)
		if !gatewayJWTConsumerVisibleToApp(consumer, secret, appID, boundToApp) {
			continue
		}
		response := gatewayJWTConsumerResponse(consumer, secret, boundRoutes)
		result = append(result, response)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// CreateGatewayJWTConsumer creates the credential Secret before the
// ApisixConsumer. If the second operation fails, only the Secret created by
// this request is rolled back.
func (g *GatewayAction) CreateGatewayJWTConsumer(ctx context.Context, namespace, appID string, req *apimodel.GatewayJWTConsumerRequest) (*apimodel.GatewayJWTConsumer, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: name is required", ErrGatewayJWTConsumerInvalidName)
	}
	if err := validateGatewayJWTConsumerName(req.Name); err != nil {
		return nil, err
	}
	secretData, generatedSecret, err := buildGatewayJWTSecretData(&req.GatewayJWTConsumerCredential)
	if err != nil {
		return nil, err
	}

	labels := managedGatewayJWTResourceLabels(appID)
	secretName := gatewayJWTConsumerSecretName(req.Name)
	secret, err := g.kubeClient.CoreV1().Secrets(namespace).Create(ctx, &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create JWT credential Secret %q: %w", secretName, err)
	}

	consumer, err := g.apisixClient.ApisixV2().ApisixConsumers(namespace).Create(ctx, &v2.ApisixConsumer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiutil.APIVersion,
			Kind:       "ApisixConsumer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: namespace,
			Labels:    managedGatewayJWTResourceLabels(appID),
		},
		Spec: v2.ApisixConsumerSpec{
			IngressClassName: gatewayJWTIngressClassName,
			AuthParameter: v2.ApisixConsumerAuthParameter{
				JwtAuth: &v2.ApisixConsumerJwtAuth{
					SecretRef: &corev1.LocalObjectReference{Name: secretName},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		rollbackErr := g.deleteCreatedGatewayJWTSecret(ctx, namespace, secret)
		if rollbackErr != nil {
			return nil, fmt.Errorf("create APISIX consumer %q: %w; rollback JWT credential Secret: %w", req.Name, err, rollbackErr)
		}
		return nil, fmt.Errorf("create APISIX consumer %q: %w", req.Name, err)
	}

	response := gatewayJWTConsumerResponse(consumer, secret, nil)
	response.GeneratedSecret = generatedSecret
	return response, nil
}

// RotateGatewayJWTConsumer replaces the credential data of an existing,
// fully Rainbond-managed Consumer without changing the ApisixConsumer.
func (g *GatewayAction) RotateGatewayJWTConsumer(ctx context.Context, namespace, name string, req *apimodel.GatewayJWTConsumerCredential) (*apimodel.GatewayJWTConsumer, error) {
	consumer, secret, err := g.managedGatewayJWTConsumerResources(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	secretData, generatedSecret, err := buildGatewayJWTSecretData(req)
	if err != nil {
		return nil, err
	}

	updatedSecret := secret.DeepCopy()
	updatedSecret.Data = secretData
	updatedSecret.Type = corev1.SecretTypeOpaque
	updatedSecret, err = g.kubeClient.CoreV1().Secrets(namespace).Update(ctx, updatedSecret, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("update JWT credential Secret %q: %w", secret.Name, err)
	}
	response := gatewayJWTConsumerResponse(consumer, updatedSecret, nil)
	response.GeneratedSecret = generatedSecret
	return response, nil
}

// DeleteGatewayJWTConsumer removes an unbound, fully Rainbond-managed
// Consumer before deleting its credential Secret.
func (g *GatewayAction) DeleteGatewayJWTConsumer(ctx context.Context, namespace, name string) error {
	if err := validateGatewayJWTConsumerName(name); err != nil {
		return err
	}
	consumer, err := g.apisixClient.ApisixV2().ApisixConsumers(namespace).Get(ctx, name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return g.deleteOrphanedGatewayJWTSecret(ctx, namespace, name)
	}
	if err != nil {
		return fmt.Errorf("get APISIX consumer %q: %w", name, err)
	}
	secret, err := g.managedGatewayJWTConsumerSecret(ctx, namespace, consumer)
	if err != nil {
		return err
	}
	routes, err := g.apisixClient.ApisixV2().ApisixRoutes(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list APISIX routes in namespace %q: %w", namespace, err)
	}
	boundRoutes, _ := gatewayJWTConsumerBindings(routes.Items, gatewayJWTConsumerUsername(namespace, name), "")
	if len(boundRoutes) > 0 {
		return fmt.Errorf("%w: %s", ErrGatewayJWTConsumerInUse, strings.Join(boundRoutes, ", "))
	}
	sharedConsumers, err := g.gatewayJWTConsumersReferencingSecret(ctx, namespace, secret.Name, &consumer.UID)
	if err != nil {
		return err
	}
	if len(sharedConsumers) > 0 {
		return fmt.Errorf("%w: JWT credential Secret %q is referenced by APISIX consumers %s", ErrGatewayJWTConsumerInUse, secret.Name, strings.Join(sharedConsumers, ", "))
	}
	if err := g.apisixClient.ApisixV2().ApisixConsumers(namespace).Delete(ctx, consumer.Name, gatewayJWTDeleteOptions(consumer.UID)); err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete APISIX consumer %q: %w", consumer.Name, err)
	}
	return g.deleteGatewayJWTSecretIfUnreferenced(ctx, namespace, secret, &consumer.UID)
}

// ConfigureManagedJWTAuth validates Consumer references and constructs the
// two route plugins owned by Rainbond. External JWT Consumers are valid route
// targets, but remain read-only through the lifecycle methods above.
func (g *GatewayAction) ConfigureManagedJWTAuth(ctx context.Context, namespace, appID string, auth *apimodel.ManagedJWTAuthentication, plugins []v2.ApisixRoutePlugin) ([]v2.ApisixRoutePlugin, error) {
	if auth == nil {
		return cloneApisixRoutePlugins(plugins), nil
	}
	if !auth.Enabled {
		return removeManagedJWTPlugins(plugins), nil
	}
	if len(auth.ConsumerNames) == 0 {
		return nil, ErrGatewayJWTConsumerRequired
	}
	if err := validateManagedJWTAuthConfig(auth.Config); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(auth.ConsumerNames))
	for _, consumerName := range auth.ConsumerNames {
		if _, exists := seen[consumerName]; exists {
			continue
		}
		seen[consumerName] = struct{}{}
		if err := validateGatewayJWTConsumerName(consumerName); err != nil {
			return nil, err
		}
		consumer, err := g.apisixClient.ApisixV2().ApisixConsumers(namespace).Get(ctx, consumerName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("get APISIX consumer %q: %w", consumerName, err)
		}
		if !gatewayJWTConsumerHasUsableAuthSource(consumer.Spec.AuthParameter.JwtAuth) {
			return nil, fmt.Errorf("%w: %s", ErrGatewayJWTConsumerNotJWT, consumerName)
		}
		if isManagedGatewayJWTResource(consumer.Labels) {
			ownerAppID := consumer.Labels[gatewayJWTApplicationLabel]
			if appID != "" && ownerAppID != "" && ownerAppID != appID {
				return nil, fmt.Errorf("%w: %s", ErrGatewayJWTConsumerAppMismatch, consumerName)
			}
		}
	}
	return BuildManagedJWTPlugins(namespace, auth.ConsumerNames, auth.Config, plugins), nil
}

// ValidateGatewayJWTConsumerCredential validates supported JWT credential
// combinations without modifying the request.
func ValidateGatewayJWTConsumerCredential(credential *apimodel.GatewayJWTConsumerCredential) error {
	if credential == nil {
		return errGatewayJWTConsumerCredentialRequired
	}
	if strings.TrimSpace(credential.Key) == "" {
		return errGatewayJWTConsumerKeyRequired
	}

	algorithm := credential.Algorithm
	if algorithm == "" {
		algorithm = gatewayJWTAlgorithmHS256
	}
	switch algorithm {
	case gatewayJWTAlgorithmHS256, gatewayJWTAlgorithmHS512:
		if credential.Secret != "" {
			if strings.TrimSpace(credential.Secret) == "" {
				return fmt.Errorf("%w: secret must not be blank", ErrGatewayJWTConsumerInvalidCredential)
			}
			secretBytes := []byte(credential.Secret)
			if credential.Base64Secret {
				decoded, err := base64.StdEncoding.Strict().DecodeString(credential.Secret)
				if err != nil || base64.StdEncoding.EncodeToString(decoded) != credential.Secret {
					return fmt.Errorf("%w: secret must be valid padded standard base64", ErrGatewayJWTConsumerInvalidCredential)
				}
				secretBytes = decoded
			}
			if len(secretBytes) < gatewayJWTMinimumSecretBytes {
				return fmt.Errorf("%w: secret must contain at least %d bytes", ErrGatewayJWTConsumerInvalidCredential, gatewayJWTMinimumSecretBytes)
			}
		}
	case gatewayJWTAlgorithmRS256:
		if strings.TrimSpace(credential.PublicKey) == "" {
			return fmt.Errorf("%w: RS256 public key is required", ErrGatewayJWTConsumerInvalidCredential)
		}
	default:
		return fmt.Errorf("%w: unsupported algorithm %q", ErrGatewayJWTConsumerInvalidCredential, algorithm)
	}

	if credential.Exp < 0 {
		return fmt.Errorf("%w: expiration must not be negative", ErrGatewayJWTConsumerInvalidCredential)
	}
	if credential.LifetimeGracePeriod < 0 {
		return fmt.Errorf("%w: lifetime grace period must not be negative", ErrGatewayJWTConsumerInvalidCredential)
	}
	return nil
}

func buildGatewayJWTSecretData(credential *apimodel.GatewayJWTConsumerCredential) (map[string][]byte, string, error) {
	if err := ValidateGatewayJWTConsumerCredential(credential); err != nil {
		return nil, "", err
	}

	algorithm := credential.Algorithm
	if algorithm == "" {
		algorithm = gatewayJWTAlgorithmHS256
	}
	secret := credential.Secret
	generatedSecret := ""
	if secret == "" && (algorithm == gatewayJWTAlgorithmHS256 || algorithm == gatewayJWTAlgorithmHS512) {
		secretBytes := make([]byte, gatewayJWTGeneratedSecretBytes)
		if _, err := rand.Read(secretBytes); err != nil {
			return nil, "", fmt.Errorf("generate JWT consumer secret: %w", err)
		}
		if credential.Base64Secret {
			secret = base64.StdEncoding.EncodeToString(secretBytes)
		} else {
			secret = base64.RawURLEncoding.EncodeToString(secretBytes)
		}
		generatedSecret = secret
	}
	expiration := credential.Exp
	if expiration == 0 {
		expiration = gatewayJWTDefaultExpiration
	}

	return map[string][]byte{
		gatewayJWTSecretDataKey:                 []byte(credential.Key),
		gatewayJWTSecretDataSecret:              []byte(secret),
		gatewayJWTSecretDataPublicKey:           []byte(credential.PublicKey),
		gatewayJWTSecretDataPrivateKey:          []byte(credential.PrivateKey),
		gatewayJWTSecretDataAlgorithm:           []byte(algorithm),
		gatewayJWTSecretDataExp:                 []byte(strconv.FormatInt(expiration, 10)),
		gatewayJWTSecretDataBase64Secret:        []byte(strconv.FormatBool(credential.Base64Secret)),
		gatewayJWTSecretDataLifetimeGracePeriod: []byte(strconv.FormatInt(credential.LifetimeGracePeriod, 10)),
	}, generatedSecret, nil
}

// BuildManagedJWTPlugins preserves ordinary plugins and replaces all existing
// managed JWT plugin instances with one deterministic pair.
func BuildManagedJWTPlugins(namespace string, consumerNames []string, config v2.ApisixRoutePluginConfig, plugins []v2.ApisixRoutePlugin) []v2.ApisixRoutePlugin {
	result := removeManagedJWTPlugins(plugins)
	result = append(result, v2.ApisixRoutePlugin{
		Name:   gatewayJWTAuthPluginName,
		Enable: true,
		Config: cloneApisixRoutePluginConfig(config),
	})

	whitelist := make([]string, 0, len(consumerNames))
	seen := make(map[string]struct{}, len(consumerNames))
	for _, consumerName := range consumerNames {
		if _, exists := seen[consumerName]; exists {
			continue
		}
		seen[consumerName] = struct{}{}
		whitelist = append(whitelist, gatewayJWTConsumerUsername(namespace, consumerName))
	}
	result = append(result, v2.ApisixRoutePlugin{
		Name:   gatewayConsumerRestrictionPluginName,
		Enable: true,
		Config: v2.ApisixRoutePluginConfig{"whitelist": whitelist},
	})
	return result
}

func removeManagedJWTPlugins(plugins []v2.ApisixRoutePlugin) []v2.ApisixRoutePlugin {
	result := make([]v2.ApisixRoutePlugin, 0, len(plugins))
	for _, plugin := range plugins {
		if plugin.Name == gatewayJWTAuthPluginName || plugin.Name == gatewayConsumerRestrictionPluginName {
			continue
		}
		plugin.Config = cloneApisixRoutePluginConfig(plugin.Config)
		result = append(result, plugin)
	}
	return result
}

func validateManagedJWTAuthConfig(config v2.ApisixRoutePluginConfig) error {
	for field, rawValue := range config {
		switch field {
		case "header", "query", "cookie":
			value, ok := rawValue.(string)
			if !ok || strings.TrimSpace(value) == "" {
				return fmt.Errorf("%w: %s must be a non-empty string", ErrGatewayJWTAuthInvalidConfig, field)
			}
		case "claims_to_verify":
			if !managedJWTClaimsAreStrings(rawValue) {
				return fmt.Errorf("%w: claims_to_verify must be an array of strings", ErrGatewayJWTAuthInvalidConfig)
			}
		case "anonymous_consumer":
			return fmt.Errorf("%w: anonymous_consumer is not supported", ErrGatewayJWTAuthInvalidConfig)
		default:
			return fmt.Errorf("%w: unsupported field %q", ErrGatewayJWTAuthInvalidConfig, field)
		}
	}
	return nil
}

func managedJWTClaimsAreStrings(rawValue interface{}) bool {
	switch claims := rawValue.(type) {
	case []string:
		for _, claim := range claims {
			if strings.TrimSpace(claim) == "" {
				return false
			}
		}
		return true
	case []interface{}:
		for _, rawClaim := range claims {
			claim, ok := rawClaim.(string)
			if !ok || strings.TrimSpace(claim) == "" {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func gatewayJWTConsumerHasUsableAuthSource(jwtAuth *v2.ApisixConsumerJwtAuth) bool {
	if jwtAuth == nil {
		return false
	}
	if jwtAuth.SecretRef != nil && strings.TrimSpace(jwtAuth.SecretRef.Name) != "" {
		return true
	}
	return jwtAuth.Value != nil && strings.TrimSpace(jwtAuth.Value.Key) != ""
}

// ManagedJWTAuthenticationFromRoute returns managed authentication metadata
// only when the route carries Rainbond's explicit ownership marker.
func ManagedJWTAuthenticationFromRoute(route *v2.ApisixRoute) *apimodel.ManagedJWTAuthentication {
	if route == nil || route.Labels[gatewayJWTManagedRouteLabel] != gatewayJWTManagedRouteValue || len(route.Spec.HTTP) == 0 {
		return nil
	}

	auth := &apimodel.ManagedJWTAuthentication{Enabled: true}
	jwtEnabled := false
	restrictionEnabled := false
	for _, plugin := range route.Spec.HTTP[0].Plugins {
		switch plugin.Name {
		case gatewayJWTAuthPluginName:
			if !plugin.Enable {
				continue
			}
			jwtEnabled = true
			auth.Config = cloneApisixRoutePluginConfig(plugin.Config)
		case gatewayConsumerRestrictionPluginName:
			if !plugin.Enable {
				continue
			}
			restrictionEnabled = true
			auth.ConsumerNames = managedJWTConsumerNames(route.Namespace, plugin.Config["whitelist"])
		}
	}
	if !jwtEnabled || !restrictionEnabled || len(auth.ConsumerNames) == 0 || validateManagedJWTAuthConfig(auth.Config) != nil {
		return nil
	}
	return auth
}

func managedJWTConsumerNames(namespace string, rawWhitelist interface{}) []string {
	prefix := namespace + "_"
	var usernames []string
	switch whitelist := rawWhitelist.(type) {
	case []string:
		usernames = whitelist
	case []interface{}:
		usernames = make([]string, 0, len(whitelist))
		for _, item := range whitelist {
			if username, ok := item.(string); ok {
				usernames = append(usernames, username)
			}
		}
	}

	consumerNames := make([]string, 0, len(usernames))
	seen := make(map[string]struct{}, len(usernames))
	for _, username := range usernames {
		if !strings.HasPrefix(username, prefix) {
			continue
		}
		consumerName := strings.TrimPrefix(username, prefix)
		if consumerName == "" {
			continue
		}
		if _, exists := seen[consumerName]; exists {
			continue
		}
		seen[consumerName] = struct{}{}
		consumerNames = append(consumerNames, consumerName)
	}
	return consumerNames
}

func gatewayJWTConsumerResponse(consumer *v2.ApisixConsumer, secret *corev1.Secret, boundRoutes []string) *apimodel.GatewayJWTConsumer {
	if consumer == nil {
		return nil
	}

	source := gatewayJWTExternalSource
	if isFullyManagedGatewayJWTConsumer(consumer, secret) {
		source = gatewayJWTManagedValue
	}
	response := &apimodel.GatewayJWTConsumer{
		Name:        consumer.Name,
		Username:    gatewayJWTConsumerUsername(consumer.Namespace, consumer.Name),
		Source:      source,
		Status:      gatewayJWTConsumerStatus(consumer.Status),
		BoundRoutes: append(make([]string, 0, len(boundRoutes)), boundRoutes...),
	}
	if isFullyManagedGatewayJWTConsumer(consumer, secret) {
		response.Key = string(secret.Data[gatewayJWTSecretDataKey])
		response.Algorithm = string(secret.Data[gatewayJWTSecretDataAlgorithm])
	}
	sort.Strings(response.BoundRoutes)
	return response
}

func (g *GatewayAction) managedGatewayJWTConsumerResources(ctx context.Context, namespace, name string) (*v2.ApisixConsumer, *corev1.Secret, error) {
	if err := validateGatewayJWTConsumerName(name); err != nil {
		return nil, nil, err
	}
	consumer, err := g.apisixClient.ApisixV2().ApisixConsumers(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("get APISIX consumer %q: %w", name, err)
	}
	secret, err := g.managedGatewayJWTConsumerSecret(ctx, namespace, consumer)
	if err != nil {
		return nil, nil, err
	}
	return consumer, secret, nil
}

func (g *GatewayAction) managedGatewayJWTConsumerSecret(ctx context.Context, namespace string, consumer *v2.ApisixConsumer) (*corev1.Secret, error) {
	if !isManagedGatewayJWTResource(consumer.Labels) {
		return nil, fmt.Errorf("%w: APISIX consumer %q", ErrGatewayJWTConsumerNotManaged, consumer.Name)
	}
	jwtAuth := consumer.Spec.AuthParameter.JwtAuth
	if jwtAuth == nil || jwtAuth.SecretRef == nil || jwtAuth.SecretRef.Name == "" {
		return nil, fmt.Errorf("%w: APISIX consumer %q has no managed JWT Secret reference", ErrGatewayJWTConsumerNotManaged, consumer.Name)
	}
	expectedSecretName := gatewayJWTConsumerSecretName(consumer.Name)
	if jwtAuth.SecretRef.Name != expectedSecretName {
		return nil, fmt.Errorf("%w: APISIX consumer %q references non-owned JWT Secret %q", ErrGatewayJWTConsumerNotManaged, consumer.Name, jwtAuth.SecretRef.Name)
	}
	secret, err := g.kubeClient.CoreV1().Secrets(namespace).Get(ctx, expectedSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get JWT credential Secret %q: %w", expectedSecretName, err)
	}
	if !isFullyManagedGatewayJWTConsumer(consumer, secret) {
		return nil, fmt.Errorf("%w: JWT credential Secret %q", ErrGatewayJWTConsumerNotManaged, secret.Name)
	}
	return secret, nil
}

func (g *GatewayAction) deleteOrphanedGatewayJWTSecret(ctx context.Context, namespace, consumerName string) error {
	secretName := gatewayJWTConsumerSecretName(consumerName)
	secret, err := g.kubeClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get orphaned JWT credential Secret %q: %w", secretName, err)
	}
	if !isManagedGatewayJWTResource(secret.Labels) {
		return fmt.Errorf("%w: orphaned JWT credential Secret %q", ErrGatewayJWTConsumerNotManaged, secretName)
	}
	return g.deleteGatewayJWTSecretIfUnreferenced(ctx, namespace, secret, nil)
}

func (g *GatewayAction) listManagedGatewayJWTSecrets(ctx context.Context, namespace string) (map[string]*corev1.Secret, error) {
	selector := k8slabels.SelectorFromSet(k8slabels.Set{
		gatewayJWTManagedByLabel: gatewayJWTManagedValue,
		gatewayJWTAuthTypeLabel:  gatewayJWTAuthTypeValue,
	}).String()
	secrets, err := g.kubeClient.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("list managed JWT credential Secrets in namespace %q: %w", namespace, err)
	}
	result := make(map[string]*corev1.Secret, len(secrets.Items))
	for i := range secrets.Items {
		secret := &secrets.Items[i]
		result[secret.Name] = secret
	}
	return result, nil
}

func (g *GatewayAction) gatewayJWTConsumersReferencingSecret(ctx context.Context, namespace, secretName string, excludedConsumerUID *types.UID) ([]string, error) {
	consumers, err := g.apisixClient.ApisixV2().ApisixConsumers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("check references to JWT credential Secret %q: %w", secretName, err)
	}
	result := make([]string, 0)
	for i := range consumers.Items {
		consumer := &consumers.Items[i]
		if excludedConsumerUID != nil && consumer.UID == *excludedConsumerUID {
			continue
		}
		jwtAuth := consumer.Spec.AuthParameter.JwtAuth
		if jwtAuth != nil && jwtAuth.SecretRef != nil && jwtAuth.SecretRef.Name == secretName {
			result = append(result, consumer.Name)
		}
	}
	sort.Strings(result)
	return result, nil
}

func (g *GatewayAction) deleteGatewayJWTSecretIfUnreferenced(ctx context.Context, namespace string, secret *corev1.Secret, excludedConsumerUID *types.UID) error {
	references, err := g.gatewayJWTConsumersReferencingSecret(ctx, namespace, secret.Name, excludedConsumerUID)
	if err != nil {
		return err
	}
	if len(references) > 0 {
		return fmt.Errorf("%w: JWT credential Secret %q is referenced by APISIX consumers %s", ErrGatewayJWTConsumerInUse, secret.Name, strings.Join(references, ", "))
	}
	if err := g.kubeClient.CoreV1().Secrets(namespace).Delete(ctx, secret.Name, gatewayJWTDeleteOptions(secret.UID)); err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete JWT credential Secret %q: %w", secret.Name, err)
	}
	return nil
}

func (g *GatewayAction) deleteCreatedGatewayJWTSecret(ctx context.Context, namespace string, secret *corev1.Secret) error {
	if err := g.kubeClient.CoreV1().Secrets(namespace).Delete(ctx, secret.Name, gatewayJWTDeleteOptions(secret.UID)); err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete request-created JWT credential Secret %q: %w", secret.Name, err)
	}
	return nil
}

func gatewayJWTDeleteOptions(uid types.UID) metav1.DeleteOptions {
	return metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}
}

func gatewayJWTConsumerBindings(routes []v2.ApisixRoute, username, appID string) ([]string, bool) {
	boundRoutes := make([]string, 0)
	boundToApp := false
	for i := range routes {
		route := &routes[i]
		if !gatewayJWTConsumerBoundToRoute(route, username) {
			continue
		}
		if appID != "" && route.Labels[gatewayJWTApplicationLabel] != appID {
			continue
		}
		boundRoutes = append(boundRoutes, route.Name)
		if appID != "" {
			boundToApp = true
		}
	}
	sort.Strings(boundRoutes)
	return boundRoutes, boundToApp
}

func gatewayJWTConsumerBoundToRoute(route *v2.ApisixRoute, username string) bool {
	if route == nil {
		return false
	}
	for i := range route.Spec.HTTP {
		for _, plugin := range route.Spec.HTTP[i].Plugins {
			if plugin.Name == gatewayConsumerRestrictionPluginName && gatewayJWTWhitelistContains(plugin.Config["whitelist"], username) {
				return true
			}
		}
	}
	return false
}

func gatewayJWTWhitelistContains(rawWhitelist interface{}, username string) bool {
	switch whitelist := rawWhitelist.(type) {
	case []string:
		for _, item := range whitelist {
			if item == username {
				return true
			}
		}
	case []interface{}:
		for _, item := range whitelist {
			if value, ok := item.(string); ok && value == username {
				return true
			}
		}
	}
	return false
}

func gatewayJWTConsumerVisibleToApp(consumer *v2.ApisixConsumer, secret *corev1.Secret, appID string, boundToApp bool) bool {
	if appID == "" || !isFullyManagedGatewayJWTConsumer(consumer, secret) {
		return true
	}
	ownerAppID := consumer.Labels[gatewayJWTApplicationLabel]
	return ownerAppID == "" || ownerAppID == appID || boundToApp
}

func isFullyManagedGatewayJWTConsumer(consumer *v2.ApisixConsumer, secret *corev1.Secret) bool {
	if consumer == nil || secret == nil || !isManagedGatewayJWTResource(consumer.Labels) || !isManagedGatewayJWTResource(secret.Labels) {
		return false
	}
	jwtAuth := consumer.Spec.AuthParameter.JwtAuth
	if jwtAuth == nil || jwtAuth.SecretRef == nil {
		return false
	}
	expectedSecretName := gatewayJWTConsumerSecretName(consumer.Name)
	return jwtAuth.SecretRef.Name == expectedSecretName && secret.Name == expectedSecretName && secret.Namespace == consumer.Namespace &&
		consumer.Labels[gatewayJWTApplicationLabel] == secret.Labels[gatewayJWTApplicationLabel]
}

func validateGatewayJWTConsumerName(name string) error {
	if validationErrors := validation.IsDNS1123Subdomain(name); len(validationErrors) > 0 {
		return fmt.Errorf("%w: %q: %s", ErrGatewayJWTConsumerInvalidName, name, strings.Join(validationErrors, "; "))
	}
	secretName := gatewayJWTConsumerSecretName(name)
	if validationErrors := validation.IsDNS1123Subdomain(secretName); len(validationErrors) > 0 {
		return fmt.Errorf("%w: generated Secret name %q: %s", ErrGatewayJWTConsumerInvalidName, secretName, strings.Join(validationErrors, "; "))
	}
	return nil
}

func gatewayJWTConsumerSecretName(consumerName string) string {
	return gatewayJWTSecretNamePrefix + consumerName
}

func managedGatewayJWTResourceLabels(appID string) map[string]string {
	labels := map[string]string{
		gatewayJWTManagedByLabel: gatewayJWTManagedValue,
		gatewayJWTAuthTypeLabel:  gatewayJWTAuthTypeValue,
	}
	if appID != "" {
		labels[gatewayJWTApplicationLabel] = appID
	}
	return labels
}

func gatewayJWTConsumerStatus(status v2.ApisixStatus) string {
	for _, condition := range status.Conditions {
		if condition.Type != "ResourcesAvailable" {
			continue
		}
		switch condition.Status {
		case metav1.ConditionTrue:
			return "ready"
		case metav1.ConditionFalse:
			return "error"
		}
	}
	return gatewayJWTConsumerStatusPending
}

func isManagedGatewayJWTResource(labels map[string]string) bool {
	return labels[gatewayJWTManagedByLabel] == gatewayJWTManagedValue && labels[gatewayJWTAuthTypeLabel] == gatewayJWTAuthTypeValue
}

func gatewayJWTConsumerUsername(namespace, consumerName string) string {
	return namespace + "_" + consumerName
}

func cloneApisixRoutePluginConfig(config v2.ApisixRoutePluginConfig) v2.ApisixRoutePluginConfig {
	if config == nil {
		return nil
	}
	return *config.DeepCopy()
}

func cloneApisixRoutePlugins(plugins []v2.ApisixRoutePlugin) []v2.ApisixRoutePlugin {
	result := make([]v2.ApisixRoutePlugin, 0, len(plugins))
	for _, plugin := range plugins {
		plugin.Config = cloneApisixRoutePluginConfig(plugin.Config)
		result = append(result, plugin)
	}
	return result
}
