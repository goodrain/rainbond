package apigateway

// Struct -
type Struct struct{}
type responseBody struct {
	Name string      `json:"name"`
	Body interface{} `json:"body"`
}

// APIVersion -
const APIVersion = "apisix.apache.org/v2"

// ApisixUpstream -
const ApisixUpstream = "ApisixUpstream"

// ApisixRoute -
const ApisixRoute = "ApisixRoute"

// ApisixTls -
const ApisixTls = "ApisixTls"

// ResponseRewrite -
const ResponseRewrite = "response-rewrite"
