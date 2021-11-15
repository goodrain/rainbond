package bcode

// ingress: 11200~11299
var (
	ErrIngressHTTPRuleNotFound = newByMessage(404, 11200, "http rule not found")
	ErrIngressTCPRuleNotFound  = newByMessage(404, 11201, "tcp rule not found")
)
