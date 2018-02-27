package goftp

// FTP Status codes, defined in RFC 959
const (
	StatusFileOK                = "150"
	StatusOK                    = "200"
	StatusSystemStatus          = "211"
	StatusDirectoryStatus       = "212"
	StatusFileStatus            = "213"
	StatusConnectionClosing     = "221"
	StatusSystemType            = "215"
	StatusClosingDataConnection = "226"
	StatusActionOK              = "250"
	StatusPathCreated           = "257"
	StatusActionPending         = "350"
)

var statusText = map[string]string{
	StatusFileOK:                "File status okay; about to open data connection",
	StatusOK:                    "Command okay",
	StatusSystemStatus:          "System status, or system help reply",
	StatusDirectoryStatus:       "Directory status",
	StatusFileStatus:            "File status",
	StatusConnectionClosing:     "Service closing control connection",
	StatusSystemType:            "System Type",
	StatusClosingDataConnection: "Closing data connection. Requested file action successful.",
	StatusActionOK:              "Requested file action okay, completed",
	StatusPathCreated:           "Pathname Created",
	StatusActionPending:         "Requested file action pending further information",
}

// StatusText returns a text for the FTP status code. It returns the empty
// string if the code is unknown.
func StatusText(code string) string {
	return statusText[code]
}
