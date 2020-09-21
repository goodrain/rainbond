package bcode

// tenant application 11000~11099
var (
	//ErrApplicationNotFound -
	ErrApplicationNotFound = newByMessage(404, 11001, "application not found")
	//ErrApplicationExist -
	ErrApplicationExist = newByMessage(400, 11002, "application already exist")
	//ErrCreateNeedCorrectAppID
	ErrCreateNeedCorrectAppID = newByMessage(404, 11003, "create service need correct application ID")
	//ErrUpdateNeedCorrectAppID
	ErrUpdateNeedCorrectAppID = newByMessage(404, 11004, "update service need correct application ID")
	//ErrDeleteDueToBindService
	ErrDeleteDueToBindService = newByMessage(400, 11005, "the application cannot be deleted because there are bound services")
)

// tenant application 11100~11199
var (
	//ErrApplicationConfigGroupExist -
	ErrApplicationConfigGroupExist = newByMessage(400, 11101, "application config group already exist")
)
