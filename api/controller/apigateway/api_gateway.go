package apigateway

// Struct -
type Struct struct{}
type responseBody struct {
	Name string      `json:"name"`
	Body interface{} `json:"body"`
}
