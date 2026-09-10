package model

import v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"

// GatewayJWTConsumerCredential contains JWT credential input. Credential
// material must never be embedded in a persisted API response.
type GatewayJWTConsumerCredential struct {
	Key                 string `json:"key" validate:"required"`
	Secret              string `json:"secret,omitempty"`
	Algorithm           string `json:"algorithm,omitempty"`
	PublicKey           string `json:"public_key,omitempty"`
	PrivateKey          string `json:"private_key,omitempty"`
	Exp                 int64  `json:"exp,omitempty"`
	Base64Secret        bool   `json:"base64_secret,omitempty"`
	LifetimeGracePeriod int64  `json:"lifetime_grace_period,omitempty"`
}

// GatewayJWTConsumerRequest creates a JWT Consumer and its credential.
type GatewayJWTConsumerRequest struct {
	Name string `json:"name" validate:"required"`
	GatewayJWTConsumerCredential
}

// GatewayJWTConsumer is the safe API representation of an APISIX Consumer.
// GeneratedSecret is populated only once when Rainbond generates a credential.
type GatewayJWTConsumer struct {
	Name            string   `json:"name"`
	Username        string   `json:"username"`
	Key             string   `json:"key,omitempty"`
	Algorithm       string   `json:"algorithm,omitempty"`
	Source          string   `json:"source"`
	Status          string   `json:"status"`
	BoundRoutes     []string `json:"bound_routes"`
	GeneratedSecret string   `json:"generated_secret,omitempty"`
}

// ManagedJWTAuthentication describes Rainbond-managed JWT route access.
type ManagedJWTAuthentication struct {
	Enabled       bool                       `json:"enabled"`
	ConsumerNames []string                   `json:"consumerNames,omitempty"`
	Config        v2.ApisixRoutePluginConfig `json:"config,omitempty"`
}

// GatewayHTTPRouteRequest extends the existing APISIX route model without
// changing the JSON accepted from legacy clients.
type GatewayHTTPRouteRequest struct {
	v2.ApisixRouteHTTP
	ManagedJWTAuth *ManagedJWTAuthentication `json:"managedJwtAuth,omitempty"`
}
