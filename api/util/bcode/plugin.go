package bcode

// plugin 11400~11499
var (
	// ErrPluginNotFound is returned when the plugin cannot be found.
	ErrPluginNotFound = newByMessage(404, 11400, "plugin not found")
)
