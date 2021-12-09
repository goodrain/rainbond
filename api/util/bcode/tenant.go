package bcode

// tenant 11300~11399
var (
	ErrNamespaceExists = newByMessage(400, 11300, "tenant namespace exists")
)
