package bcode

// service: 10000~10099
var (
	//ErrPortNotFound -
	ErrPortNotFound = newByMessage(404, 10001, "service port not found")
	//ErrServiceMonitorNotFound -
	ErrServiceMonitorNotFound = newByMessage(404, 10101, "service monitor not found")
	//ErrServiceMonitorNameExist -
	ErrServiceMonitorNameExist = newByMessage(400, 10102, "service monitor name is exist")
	// ErrSyncOperation -
	ErrSyncOperation = newByMessage(409, 10103, "The asynchronous operation is executing")
	// ErrHorizontalDueToNoChange
	ErrHorizontalDueToNoChange = newByMessage(400, 10104, "The number of components has not changed, no need to scale")
	ErrPodNotFound             = newByMessage(404, 10105, "pod not found")
	ErrK8sComponentNameExists  = newByMessage(400, 10106, "k8s component name exists")
)
