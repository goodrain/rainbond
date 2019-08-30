package event

// GetLoggerOption -
func GetLoggerOption(status string) map[string]string {
	return map[string]string{"step": "appruntime", "status": status}
}

//GetCallbackLoggerOption get callback logger
func GetCallbackLoggerOption() map[string]string {
	return map[string]string{"step": "callback", "status": "failure"}
}

//GetTimeoutLoggerOption get callback logger
func GetTimeoutLoggerOption() map[string]string {
	return map[string]string{"step": "callback", "status": "timeout"}
}

//GetLastLoggerOption get last logger
func GetLastLoggerOption() map[string]string {
	return map[string]string{"step": "last", "status": "success"}
}
