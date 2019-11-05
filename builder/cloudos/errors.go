package cloudos

import "fmt"

// S3SDKError -
type S3SDKError struct {
	Code       string // The error code returned from S3 to the caller
	Message    string // The detail error message from S3
	RawMessage string // The raw messages from S3
	StatusCode int    // HTTP status code
}

// Error implements interface error
func (e S3SDKError) Error() string {
	return fmt.Sprintf("s3: service returned error: StatusCode=%d, ErrorCode=%s, ErrorMessage=\"%s\"",
		e.StatusCode, e.Code, e.Message)
}
