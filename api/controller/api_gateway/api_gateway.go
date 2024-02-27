package api_gateway

// APIGatewayStruct -
type APIGatewayStruct struct{}
type responseBody struct {
	Name string      `json:"name"`
	Body interface{} `json:"body"`
}
