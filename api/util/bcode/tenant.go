package bcode

// tenant 11300~11399
var (
	// ErrNamespaceExists is returned when the tenant namespace already exists.
	ErrNamespaceExists = newByMessage(400, 11300, "tenant namespace exists")
	// ErrTenantNotFound is returned when the tenant cannot be found.
	ErrTenantNotFound = newByMessage(404, 11301, "tenant not found")
)
