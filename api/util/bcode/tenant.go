package bcode

// tenant 11300~11399
var (
	// ErrTenantNotFound is returned when the tenant cannot be found.
	ErrTenantNotFound = newByMessage(404, 11300, "tenant not found")
	// ErrNamespaceExists is returned when the tenant namespace already exists.
	ErrNamespaceExists = newByMessage(400, 11301, "tenant namespace exists")
)
