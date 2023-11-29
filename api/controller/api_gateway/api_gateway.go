package api_gateway

type APIGatewayStruct struct{}
type responseBody struct {
	Name string      `json:"name"`
	Body interface{} `json:"body"`
}
