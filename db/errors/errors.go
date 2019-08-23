package errors

import (
	"errors"
)

var (
	// ErrRecordAlreadyExist record already exist error, happens when find any matched data when creating with a struct
	ErrRecordAlreadyExist = errors.New("record already exist")
)
